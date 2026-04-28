package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"
)

const banner = `
  _                        _____
 | |  _  _ _ _  __ _ _ _  |_   _|___ __ _ _ _
 | |_| || | ' \/ _` + "`" + ` | '_|   | |/ -_)/ _` + "`" + ` | '_|
 |____\_,_|_||_\__,_|_|     |_|\___|\__,_|_|

`

const (
	configFile = ".wizard.json"
	exitVal    = "__exit__"
)

type config struct {
	IP       string `json:"ip"`
	Device   string `json:"device"`
	Detail   string `json:"detail"`
	Summary  string `json:"summary"`
	GRPCPort int    `json:"grpc_port,omitempty"`
	CDNPort  int    `json:"cdn_port,omitempty"`
	AuthPort int    `json:"auth_port,omitempty"`
}

const (
	defaultGRPCPort = 8003
	defaultCDNPort  = 8080
	defaultAuthPort = 3000
)

type ports struct {
	GRPC int
	CDN  int
	Auth int
}

func main() {
	setupOnly := flag.Bool("setup-only", false, "show patching instructions and exit without building or launching")
	preferSaved := flag.Bool("prefer-saved", false, "reuse saved config without prompting")
	grpcPort := flag.Int("grpc-port", defaultGRPCPort, "gRPC server port")
	cdnPort := flag.Int("cdn-port", defaultCDNPort, "CDN server port")
	authPort := flag.Int("auth-port", defaultAuthPort, "auth server port")
	flag.Parse()

	flagSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })

	lipgloss.EnableLegacyWindowsANSI(os.Stdout)
	lipgloss.EnableLegacyWindowsANSI(os.Stderr)

	fmt.Print(banner)

	if !*setupOnly {
		validateAssets()
		validateTools()
		validateProtocIncludes()
		runProtoc()
		runMigrate()
		downloadDeps()
	}

	ip, cfg, firstRun := resolveIP(*preferSaved)

	p := resolvePorts(flagSet, *grpcPort, *cdnPort, *authPort, cfg)
	savedPorts := portsFromConfig(cfg)

	if !firstRun && (p.GRPC != savedPorts.GRPC || p.CDN != savedPorts.CDN || p.Auth != savedPorts.Auth) {
		if !warnPortChange(savedPorts, p) {
			os.Exit(0)
		}
	}

	cfg.GRPCPort = p.GRPC
	cfg.CDNPort = p.CDN
	cfg.AuthPort = p.Auth
	saveConfig(cfg)

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(14)
	addrStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	fmt.Println()
	fmt.Printf("  %s %s\n", labelStyle.Render("Game server:"), addrStyle.Render(fmt.Sprintf("%s:%d", ip, p.GRPC)))
	fmt.Printf("  %s %s\n", labelStyle.Render("CDN:"), addrStyle.Render(fmt.Sprintf("%s:%d", ip, p.CDN)))
	fmt.Printf("  %s %s\n", labelStyle.Render("Auth:"), addrStyle.Render(fmt.Sprintf("%s:%d", ip, p.Auth)))
	fmt.Println()

	if firstRun || *setupOnly {
		showPatcherHint(ip, p, !*setupOnly)
	}

	if *setupOnly {
		return
	}

	launchDev(ip, p)
}

type assetCheck struct {
	path string
	dir  bool
}

var requiredAssets = []assetCheck{
	{"assets", true},
	{"assets/release/database.bin.e", false},
}

// assetTrees lists every rev-0 layout the CDN accepts. At least one full set
// must be present; users may have only the un-split shared tree, only Android,
// only iOS, or any combination.
var assetTrees = [][]assetCheck{
	{
		{"assets/revisions/0/list.bin", false},
		{"assets/revisions/0/assetbundle", true},
		{"assets/revisions/0/resources", true},
	},
	{
		{"assets/revisions/0/android/list.bin", false},
		{"assets/revisions/0/android/assetbundle", true},
		{"assets/revisions/0/android/resources", true},
	},
	{
		{"assets/revisions/0/ios/list.bin", false},
		{"assets/revisions/0/ios/assetbundle", true},
		{"assets/revisions/0/ios/resources", true},
	},
}

func checkAsset(a assetCheck) (missing string, ok bool) {
	info, err := os.Stat(a.path)
	if err != nil {
		return a.path, false
	}
	if a.dir && !info.IsDir() {
		return a.path + string(filepath.Separator), false
	}
	if !a.dir && info.IsDir() {
		return a.path, false
	}
	return "", true
}

func validateAssets() {
	var missing []string
	for _, a := range requiredAssets {
		if m, ok := checkAsset(a); !ok {
			missing = append(missing, m)
		}
	}

	var treeMissing [][]string
	anyTreeOK := false
	for _, group := range assetTrees {
		var groupMissing []string
		for _, a := range group {
			if m, ok := checkAsset(a); !ok {
				groupMissing = append(groupMissing, m)
			}
		}
		if len(groupMissing) == 0 {
			anyTreeOK = true
		}
		treeMissing = append(treeMissing, groupMissing)
	}
	if !anyTreeOK {
		for _, gm := range treeMissing {
			missing = append(missing, gm...)
		}
	}

	if len(missing) == 0 {
		return
	}

	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hlStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	var b strings.Builder
	b.WriteString(errStyle.Render("  Required game assets are missing."))
	b.WriteString("\n\n")
	for _, p := range missing {
		b.WriteString(pathStyle.Render("    ✗ " + p))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Place the extracted game assets under server/assets/ and try again."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  At least one of assets/revisions/0/, assets/revisions/0/android/, or assets/revisions/0/ios/ must be fully present."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Get them from ") + hlStyle.Render("#resources") + dimStyle.Render(" on Discord: ") + hlStyle.Hyperlink("https://discord.com/invite/MZAf5aVkJG").Render("https://discord.com/invite/MZAf5aVkJG"))
	b.WriteString("\n")

	fmt.Fprintln(os.Stderr, b.String())
	os.Exit(1)
}

type toolReq struct {
	bin       string
	install   string // human-readable hint shown when tool must be installed manually
	goInstall string // `go install` package path; non-empty means auto-installable
}

var requiredTools = []toolReq{
	{"make", "https://www.gnu.org/software/make/", ""},
	{"protoc", "https://protobuf.dev/installation/", ""},
	{"protoc-gen-go", "", "google.golang.org/protobuf/cmd/protoc-gen-go@latest"},
	{"protoc-gen-go-grpc", "", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"},
	{"goose", "", "github.com/pressly/goose/v3/cmd/goose@latest"},
}

var toolPaths = map[string]string{}

// findTool looks for a tool on PATH first, then falls back to the current
// directory (for Windows users who drop .exe files into server/).
func findTool(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	local := name
	if runtime.GOOS == "windows" {
		local += ".exe"
	}
	if _, err := os.Stat(local); err == nil {
		abs, err := filepath.Abs(local)
		if err != nil {
			return local, nil
		}
		return abs, nil
	}
	return "", fmt.Errorf("%s not found", name)
}

func validateTools() {
	var manual []toolReq
	var installable []toolReq

	for _, t := range requiredTools {
		if p, err := findTool(t.bin); err == nil {
			toolPaths[t.bin] = p
		} else if t.goInstall == "" {
			manual = append(manual, t)
		} else {
			installable = append(installable, t)
		}
	}

	if len(manual) > 0 {
		errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		var b strings.Builder
		b.WriteString(errStyle.Render("  Required tools are not installed."))
		b.WriteString("\n\n")
		for _, t := range manual {
			b.WriteString(nameStyle.Render(fmt.Sprintf("    ✗ %-22s", t.bin)) + hintStyle.Render(t.install))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		fmt.Fprintln(os.Stderr, b.String())
		os.Exit(1)
	}

	for _, t := range installable {
		fmt.Printf("  Installing %s...\n", t.bin)
		cmd := exec.Command("go", "install", t.goInstall)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to install %s: %v\n", t.bin, err)
			os.Exit(1)
		}
		p, err := findTool(t.bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s installed but not found on PATH — is $(go env GOPATH)/bin in your PATH?\n", t.bin)
			os.Exit(1)
		}
		toolPaths[t.bin] = p
	}
}

func validateProtocIncludes() {
	if _, err := exec.LookPath("protoc"); err == nil {
		return
	}
	// protoc is local (not on PATH) -- verify well-known types are present.
	wkt := filepath.Join("google", "protobuf", "empty.proto")
	if _, err := os.Stat(wkt); err == nil {
		return
	}

	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var b strings.Builder
	b.WriteString(errStyle.Render("  protoc well-known types are missing."))
	b.WriteString("\n\n")
	b.WriteString(pathStyle.Render("    ✗ " + wkt))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Extract the google/ folder from the protoc release zip into server/."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Download: https://github.com/protocolbuffers/protobuf/releases"))
	b.WriteString("\n")

	fmt.Fprintln(os.Stderr, b.String())
	os.Exit(1)
}

func runProtoc() {
	_ = spinner.New().Title("  Running protoc...").Action(func() {
		runQuiet(exec.Command(toolPaths["make"], "proto", "PROTOC="+toolPaths["protoc"]), "protoc generation")
	}).Run()
}

func runMigrate() {
	_ = spinner.New().Title("  Running migrations...").Action(func() {
		if err := os.MkdirAll("db", 0755); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to create db/: %v\n", err)
			os.Exit(1)
		}
		runQuiet(exec.Command(toolPaths["goose"], "-dir", "migrations", "-allow-missing", "sqlite3", "db/game.db", "up"), "database migration")
	}).Run()
}

func downloadDeps() {
	_ = spinner.New().Title("  Downloading dependencies...").Action(func() {
		runQuiet(exec.Command("go", "mod", "download"), "dependency download")
	}).Run()
}

func runQuiet(cmd *exec.Cmd, label string) {
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, buf.String())
		fmt.Fprintf(os.Stderr, "\n  %s failed: %v\n", label, err)
		os.Exit(1)
	}
}

func resolveIP(preferSaved bool) (string, config, bool) {
	cfg, err := loadConfig()
	if err == nil {
		if preferSaved {
			if isLANBased(cfg) {
				if ip, updated, ok := recheckLANIP(cfg); ok {
					return ip, updated, false
				}
			}
			return cfg.IP, cfg, false
		}

		ip, cfg, done := handleSavedConfig(cfg)
		if done {
			return ip, cfg, false
		}
	} else if preferSaved {
		fmt.Fprintln(os.Stderr, "  --prefer-saved: no saved config found; run without the flag first.")
		os.Exit(1)
	}

	ip, cfg := runWizard()
	return ip, cfg, true
}

func handleSavedConfig(cfg config) (string, config, bool) {
	reuse := true
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Use same settings as last time? (%s — %s)", cfg.IP, cfg.Summary)).
		Affirmative("Yes").
		Negative("No, reconfigure").
		Value(&reuse).
		Run()
	if err != nil {
		os.Exit(1)
	}
	if !reuse {
		return "", config{}, false
	}

	if isLANBased(cfg) {
		if ip, updated, ok := recheckLANIP(cfg); ok {
			return ip, updated, true
		}
		return "", config{}, false
	}

	return cfg.IP, cfg, true
}

func isLANBased(cfg config) bool {
	if cfg.Detail == "wifi" {
		return true
	}
	switch cfg.Detail {
	case "android-studio", "bluestacks", "genymotion":
		return false
	}
	return cfg.Device == "emulator"
}

func recheckLANIP(cfg config) (string, config, bool) {
	current := detectLANIP()
	if current == "" || current == cfg.IP {
		return cfg.IP, cfg, true
	}

	var action string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Your LAN IP changed: %s → %s", cfg.IP, current)).
		Options(
			huh.NewOption("Use new IP ("+current+")", "update"),
			huh.NewOption("Keep saved IP ("+cfg.IP+")", "keep"),
			huh.NewOption("Reconfigure from scratch", "reconfig"),
		).
		Value(&action).
		Run()
	if err != nil {
		os.Exit(1)
	}

	switch action {
	case "update":
		warnRepatch(cfg.IP, current, portsFromConfig(cfg))
		var ack bool
		_ = huh.NewConfirm().
			Title("Continue launching the server?").
			Affirmative("Yes, start").
			Negative("No, exit").
			Value(&ack).
			Run()
		if !ack {
			os.Exit(0)
		}
		cfg.IP = current
		return current, cfg, true
	case "keep":
		return cfg.IP, cfg, true
	default:
		return "", config{}, false
	}
}

func warnPortChange(old, new ports) bool {
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hlStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	portLine := func(label string, oldP, newP int) string {
		if oldP == newP {
			return dimStyle.Render(fmt.Sprintf("    %-7s %d (unchanged)", label+":", oldP))
		}
		return hlStyle.Render(fmt.Sprintf("    %-7s %d → %d", label+":", oldP, newP))
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render("  ⚠ Port configuration changed from last run."))
	b.WriteString("\n\n")
	b.WriteString(portLine("gRPC", old.GRPC, new.GRPC))
	b.WriteString("\n")
	b.WriteString(portLine("CDN", old.CDN, new.CDN))
	b.WriteString("\n")
	b.WriteString(portLine("Auth", old.Auth, new.Auth))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Your APK was patched for the old ports. You may need to re-patch."))
	b.WriteString("\n\n")
	fmt.Print(b.String())

	cont := true
	err := huh.NewConfirm().
		Title("Continue with new ports?").
		Affirmative("Yes, continue").
		Negative("No, exit").
		Value(&cont).
		Run()
	if err != nil {
		os.Exit(1)
	}
	return cont
}

func warnRepatch(oldIP, newIP string, p ports) {
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hlStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	repoURL := "https://gitlab.com/walter-sparrow-group/lunar-scripts"
	repoLink := hlStyle.Hyperlink(repoURL).Render(repoURL)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render("  ⚠ Your APK was patched for the old IP."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Re-patch using ") + hlStyle.Render("lunar_tear_patcher.ipynb") + dimStyle.Render(" in ") + hlStyle.Render("Google Colab") + dimStyle.Render(":"))
	b.WriteString("\n\n")
	b.WriteString("    " + repoLink)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Update the Configuration cell with the new addresses:"))
	b.WriteString("\n\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    grpc_addr = \"%s:%d\"", newIP, p.GRPC)))
	b.WriteString("\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    http_addr = \"%s:%d\"", newIP, p.CDN)))
	b.WriteString("\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    auth_host = \"%s:%d\"", newIP, p.Auth)))
	b.WriteString("\n\n")
	fmt.Print(b.String())
}

func showPatcherHint(ip string, p ports, askLaunch bool) {
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hlStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	repoURL := "https://gitlab.com/walter-sparrow-group/lunar-scripts"
	discordURL := "https://discord.com/invite/MZAf5aVkJG"
	repoLink := hlStyle.Hyperlink(repoURL).Render(repoURL)
	discordLink := hlStyle.Hyperlink(discordURL).Render(discordURL)

	var b strings.Builder
	b.WriteString(headStyle.Render("  Next step: patch your APK"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  The game client must be patched to connect to your server."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Open ") + hlStyle.Render("lunar_tear_patcher.ipynb") + dimStyle.Render(" from the scripts repo in ") + hlStyle.Render("Google Colab") + dimStyle.Render(":"))
	b.WriteString("\n\n")
	b.WriteString("    " + repoLink)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Get the APK and master data links from ") + hlStyle.Render("#resources") + dimStyle.Render(" on Discord:"))
	b.WriteString("\n\n")
	b.WriteString("    " + discordLink)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Set these in the notebook's Configuration cell:"))
	b.WriteString("\n\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    grpc_addr = \"%s:%d\"", ip, p.GRPC)))
	b.WriteString("\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    http_addr = \"%s:%d\"", ip, p.CDN)))
	b.WriteString("\n")
	b.WriteString(hlStyle.Render(fmt.Sprintf("    auth_host = \"%s:%d\"", ip, p.Auth)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Then run all cells — a patched APK will download automatically."))
	b.WriteString("\n\n")
	fmt.Print(b.String())

	if !askLaunch {
		return
	}

	launch := true
	_ = huh.NewConfirm().
		Title("Launch the server?").
		Affirmative("Yes, start").
		Negative("No, exit").
		Value(&launch).
		Run()
	if !launch {
		os.Exit(0)
	}
}

func runWizard() (string, config) {
	var device, emu, conn string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where is the game running?").
				Options(
					huh.NewOption("Android Emulator on this PC", "emulator"),
					huh.NewOption("Phone / Tablet on the same network", "phone"),
					huh.NewOption("Exit", exitVal),
				).
				Value(&device),
		).WithShowHelp(true),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which emulator are you using? ").
				Options(
					huh.NewOption("Android Studio Emulator", "android-studio"),
					huh.NewOption("BlueStacks", "bluestacks"),
					huh.NewOption("Genymotion", "genymotion"),
					huh.NewOption("Nox / LDPlayer / MEmu", "nox-ld-memu"),
					huh.NewOption("Other / Not sure", "other"),
					huh.NewOption("Exit", exitVal),
				).
				Value(&emu),
		).WithHideFunc(func() bool { return device != "emulator" }).WithShowHelp(true),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How is your phone connected to this PC?").
				Options(
					huh.NewOption("Same Wi-Fi network", "wifi"),
					huh.NewOption("Tailscale / ZeroTier / VPN", "vpn"),
					huh.NewOption("Something else / I'll type the IP", "manual"),
					huh.NewOption("Exit", exitVal),
				).
				Value(&conn),
		).WithHideFunc(func() bool { return device != "phone" }).WithShowHelp(true),
	)

	if err := form.Run(); err != nil {
		os.Exit(1)
	}

	if device == exitVal || emu == exitVal || conn == exitVal {
		os.Exit(0)
	}

	return buildResult(device, emu, conn)
}

func buildResult(device, emu, conn string) (string, config) {
	if device == "emulator" {
		return buildEmulatorResult(emu)
	}
	return buildPhoneResult(conn)
}

func buildEmulatorResult(emu string) (string, config) {
	switch emu {
	case "android-studio", "bluestacks":
		return "10.0.2.2", config{
			IP: "10.0.2.2", Device: "emulator", Detail: emu,
			Summary: emu + " emulator",
		}
	case "genymotion":
		return "10.0.3.2", config{
			IP: "10.0.3.2", Device: "emulator", Detail: emu,
			Summary: "Genymotion emulator",
		}
	default:
		ip := detectAndConfirmIP(detectLANIP(), "emulator (LAN IP)")
		return ip, config{
			IP: ip, Device: "emulator", Detail: emu,
			Summary: emu + " emulator (LAN IP)",
		}
	}
}

func buildPhoneResult(conn string) (string, config) {
	switch conn {
	case "wifi":
		ip := detectAndConfirmIP(detectLANIP(), "Wi-Fi")
		return ip, config{IP: ip, Device: "phone", Detail: "wifi", Summary: "phone (Wi-Fi)"}
	case "vpn":
		ip := detectAndConfirmIP(detectVPNIP(), "VPN")
		return ip, config{IP: ip, Device: "phone", Detail: "vpn", Summary: "phone (VPN)"}
	default:
		ip := promptIP("")
		return ip, config{IP: ip, Device: "phone", Detail: "manual", Summary: "phone (manual IP)"}
	}
}

func detectAndConfirmIP(detected, label string) string {
	if detected == "" {
		fmt.Printf("  Could not auto-detect your %s IP address.\n", label)
		return promptIP("")
	}
	return promptIP(detected)
}

func promptIP(defaultVal string) string {
	ip := defaultVal

	title := "Enter your PC's IP address"
	if defaultVal != "" {
		title = fmt.Sprintf("Enter your PC's IP address (detected: %s)", defaultVal)
	}

	err := huh.NewInput().
		Title(title).
		Description("Press Enter to accept, or type a different address.").
		Validate(func(s string) error {
			if net.ParseIP(strings.TrimSpace(s)) == nil {
				return fmt.Errorf("not a valid IP address")
			}
			return nil
		}).
		Value(&ip).
		Run()
	if err != nil {
		os.Exit(1)
	}
	return strings.TrimSpace(ip)
}

func detectLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	skipPrefixes := []string{"docker", "veth", "br-", "virbr", "tailscale", "zt", "tun", "utun"}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func detectVPNIP() string {
	if path, err := exec.LookPath("tailscale"); err == nil {
		out, err := exec.Command(path, "ip", "-4").Output()
		if err == nil {
			ip := strings.TrimSpace(string(out))
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	_, tailscaleNet, _ := net.ParseCIDR("100.64.0.0/10")
	vpnPrefixes := []string{"tailscale", "zt", "tun", "utun", "wg"}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		isVPN := false
		for _, prefix := range vpnPrefixes {
			if strings.HasPrefix(name, prefix) {
				isVPN = true
				break
			}
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			if isVPN || tailscaleNet.Contains(ip) {
				return ip.String()
			}
		}
	}
	return ""
}

func loadConfig() (config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, err
	}
	if cfg.IP == "" {
		return config{}, fmt.Errorf("empty config")
	}
	return cfg, nil
}

func portsFromConfig(cfg config) ports {
	p := ports{GRPC: cfg.GRPCPort, CDN: cfg.CDNPort, Auth: cfg.AuthPort}
	if p.GRPC == 0 {
		p.GRPC = defaultGRPCPort
	}
	if p.CDN == 0 {
		p.CDN = defaultCDNPort
	}
	if p.Auth == 0 {
		p.Auth = defaultAuthPort
	}
	return p
}

func resolvePorts(flagSet map[string]bool, grpcFlag, cdnFlag, authFlag int, saved config) ports {
	resolve := func(name string, flagVal, savedVal, defaultVal int) int {
		if flagSet[name] {
			return flagVal
		}
		if savedVal != 0 {
			return savedVal
		}
		return defaultVal
	}
	return ports{
		GRPC: resolve("grpc-port", grpcFlag, saved.GRPCPort, defaultGRPCPort),
		CDN:  resolve("cdn-port", cdnFlag, saved.CDNPort, defaultCDNPort),
		Auth: resolve("auth-port", authFlag, saved.AuthPort, defaultAuthPort),
	}
}

func saveConfig(cfg config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(configFile, append(data, '\n'), 0644)
}

func launchDev(ip string, p ports) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	devBin := filepath.Join("bin", "dev"+ext)

	_ = spinner.New().Title("  Building services...").Action(func() {
		if err := os.MkdirAll("bin", 0755); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to create bin/: %v\n", err)
			os.Exit(1)
		}
		runQuiet(exec.Command("go", "build", "-o", devBin, "./cmd/dev"), "build dev")
	}).Run()

	cmd := exec.Command(devBin,
		"--grpc.listen", fmt.Sprintf("0.0.0.0:%d", p.GRPC),
		"--grpc.public-addr", fmt.Sprintf("%s:%d", ip, p.GRPC),
		"--cdn.listen", fmt.Sprintf("0.0.0.0:%d", p.CDN),
		"--cdn.public-addr", fmt.Sprintf("%s:%d", ip, p.CDN),
		"--auth.listen", fmt.Sprintf("0.0.0.0:%d", p.Auth),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start dev server: %v\n", err)
		os.Exit(1)
	}

	go func() {
		<-sigCh
		_ = cmd.Process.Signal(os.Interrupt)
	}()

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
