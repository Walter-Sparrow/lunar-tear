#!/usr/bin/env python3
"""
patch_apk.py — Static patcher for NieR Re[in]carnation APK.

Patches an apktool-decompiled APK directory so the game connects to a
private server without any runtime (Frida) hooks.

Patches applied:
  1. global-metadata.dat  — rewrite IL2CPP string literals (URLs + hostname)
  2. libil2cpp.so          — ARM64 binary patches (SSL bypass, encryption passthrough, Octo plain list)
  3. AndroidManifest.xml  — add networkSecurityConfig for cleartext HTTP
  4. res/xml/network_security_config.xml — allow cleartext traffic
"""

import argparse
import os
import struct
import sys

# ---------------------------------------------------------------------------
# global-metadata.dat string literal patching
# ---------------------------------------------------------------------------

METADATA_MAGIC = 0xFAB11BAF

# Header offsets (v24): each section is a (uint32 offset, uint32 size) pair
HDR_STRING_LITERAL_OFF = 8      # stringLiteral table
HDR_STRING_LITERAL_DATA_OFF = 16  # stringLiteralData blob


def patch_metadata_strings(meta_path: str, replacements: list[tuple[str, str]]) -> int:
    with open(meta_path, "rb") as f:
        data = bytearray(f.read())

    magic = struct.unpack_from("<I", data, 0)[0]
    if magic != METADATA_MAGIC:
        print(f"  [!] Bad magic 0x{magic:08X}, expected 0x{METADATA_MAGIC:08X}")
        return 0

    version = struct.unpack_from("<i", data, 4)[0]
    print(f"  metadata v{version}, {len(data)} bytes")

    sl_off, sl_size = struct.unpack_from("<II", data, HDR_STRING_LITERAL_OFF)
    sld_off, sld_size = struct.unpack_from("<II", data, HDR_STRING_LITERAL_DATA_OFF)
    n_entries = sl_size // 8
    print(f"  stringLiteral: {n_entries} entries @ 0x{sl_off:X}")
    print(f"  stringLiteralData: {sld_size} bytes @ 0x{sld_off:X}")

    patched = 0
    for old_str, new_str in replacements:
        old_bytes = old_str.encode("utf-8")
        new_bytes = new_str.encode("utf-8")

        if len(new_bytes) > len(old_bytes):
            print(f"  [!] SKIP: replacement longer than original "
                  f"({len(new_bytes)} > {len(old_bytes)}): {old_str!r}")
            continue

        # Find the string in the data blob
        blob_pos = data.find(old_bytes, sld_off, sld_off + sld_size)
        if blob_pos < 0:
            print(f"  [!] NOT FOUND in blob: {old_str!r}")
            continue

        data_index = blob_pos - sld_off

        # Find the StringLiteral table entry that references this exact string
        entry_found = False
        for i in range(n_entries):
            e_off = sl_off + i * 8
            e_len, e_idx = struct.unpack_from("<II", data, e_off)
            if e_idx == data_index and e_len == len(old_bytes):
                # Update length
                struct.pack_into("<I", data, e_off, len(new_bytes))
                entry_found = True
                print(f"  entry #{i}: length {e_len} -> {len(new_bytes)}")
                break

        if not entry_found:
            print(f"  [!] No table entry found for {old_str!r} (dataIndex=0x{data_index:X})")
            continue

        # Overwrite string data (pad remainder with null bytes)
        data[blob_pos : blob_pos + len(old_bytes)] = (
            new_bytes + b"\x00" * (len(old_bytes) - len(new_bytes))
        )

        print(f"  PATCHED: {old_str!r} -> {new_str!r}")
        patched += 1

    with open(meta_path, "wb") as f:
        f.write(data)

    return patched


# ---------------------------------------------------------------------------
# AndroidManifest.xml  — add networkSecurityConfig attribute
# ---------------------------------------------------------------------------

def patch_manifest(manifest_path: str) -> bool:
    with open(manifest_path, "r", encoding="utf-8") as f:
        text = f.read()

    if "networkSecurityConfig" in text:
        print("  already has networkSecurityConfig")
        return True

    # Insert android:networkSecurityConfig="@xml/network_security_config"
    # right after the opening <application tag's first attribute
    new_attr = 'android:networkSecurityConfig="@xml/network_security_config"'
    text = text.replace("<application ", f"<application {new_attr} ", 1)

    with open(manifest_path, "w", encoding="utf-8") as f:
        f.write(text)

    print(f"  added {new_attr}")
    return True


# ---------------------------------------------------------------------------
# res/xml/network_security_config.xml
# ---------------------------------------------------------------------------

NETWORK_SECURITY_CONFIG = """\
<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
    <base-config cleartextTrafficPermitted="true" />
</network-security-config>
"""


def create_network_security_config(res_xml_dir: str) -> bool:
    os.makedirs(res_xml_dir, exist_ok=True)
    out = os.path.join(res_xml_dir, "network_security_config.xml")
    with open(out, "w", encoding="utf-8") as f:
        f.write(NETWORK_SECURITY_CONFIG)
    print(f"  wrote {out}")
    return True


# ---------------------------------------------------------------------------
# libil2cpp.so ARM64 binary patches
# ---------------------------------------------------------------------------

# ARM64 instruction encodings (little-endian)
MOV_X0_0 = struct.pack("<I", 0xD2800000)   # mov x0, #0
MOV_X0_X1 = struct.pack("<I", 0xAA0103E0)  # mov x0, x1
RET = struct.pack("<I", 0xD65F03C0)        # ret

# RVAs from dump.cs (Il2CppDumper output matching client/3.7.1.apk)
IL2CPP_PATCHES = [
    {
        "name": "ToNativeCredentials",
        "desc": "SSL bypass — return NULL to force insecure gRPC channel",
        "rva": 0x35C8670,
        "bytes": MOV_X0_0 + RET,
    },
    {
        "name": "HandleNet.Encrypt",
        "desc": "encryption passthrough — return payload as-is",
        "rva": 0x279410C,
        "bytes": MOV_X0_X1 + RET,
    },
    {
        "name": "HandleNet.Decrypt",
        "desc": "decryption passthrough — return receivedMessage as-is",
        "rva": 0x279420C,
        "bytes": MOV_X0_X1 + RET,
    },
    {
        "name": "OctoManager.Internal.GetListAes",
        "desc": "Octo list: force plain list (return false = no AES); server serves raw list.bin",
        "rva": 0x4C27038,
        "bytes": MOV_X0_0 + RET,
    },
]


def patch_libil2cpp(so_path: str) -> int:
    with open(so_path, "r+b") as f:
        file_size = f.seek(0, 2)
        patched = 0
        for p in IL2CPP_PATCHES:
            rva = p["rva"]
            if rva + len(p["bytes"]) > file_size:
                print(f"  [!] SKIP {p['name']}: RVA 0x{rva:X} beyond file size")
                continue

            # Read original bytes for verification (should be a function prologue)
            f.seek(rva)
            orig = f.read(len(p["bytes"]))

            f.seek(rva)
            f.write(p["bytes"])
            patched += 1
            print(f"  {p['name']} @ 0x{rva:X}: {orig.hex()} -> {p['bytes'].hex()}")
            print(f"    {p['desc']}")

    return patched


# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

def main():
    p = argparse.ArgumentParser(description="Patch decompiled APK for private server")
    p.add_argument("apk_dir", help="Path to apktool-decompiled APK directory")
    p.add_argument("--server-ip", required=True, help="Server IP (e.g. 10.0.2.2)")
    p.add_argument("--http-port", type=int, default=8080, help="HTTP port (default 8080)")
    p.add_argument("--grpc-port", type=int, default=7777, help="gRPC port (default 7777)")
    args = p.parse_args()

    apk = args.apk_dir.rstrip("/")
    ip = args.server_ip
    hp = args.http_port
    gp = args.grpc_port

    meta = os.path.join(apk, "assets/bin/Data/Managed/Metadata/global-metadata.dat")
    so = os.path.join(apk, "lib/arm64-v8a/libil2cpp.so")
    manifest = os.path.join(apk, "AndroidManifest.xml")
    res_xml = os.path.join(apk, "res/xml")

    for path in (meta, so, manifest):
        if not os.path.isfile(path):
            sys.exit(f"[!] Not found: {path}")

    web_url = f"http://{ip}:{hp}"
    # gRPC host only — no port. Client uses separate _serverPort (often 443); if we put
    # "ip:7777" here, it gets used as hostname and DNS is asked for "192.168.0.199:7777:443".
    grpc_host = ip

    replacements = [
        ("api.app.nierreincarnation.com", grpc_host),
        ("https://web.app.nierreincarnation.com/assets/release/{0}/database.bin",
         f"{web_url}/assets/release/{{0}}/database.bin"),
        ("https://web.app.nierreincarnation.com", web_url),
        ("https://resources-api.app.nierreincarnation.com/", f"{web_url}/"),
    ]

    # Validate that no replacement is too long
    for old, new in replacements:
        if len(new.encode("utf-8")) > len(old.encode("utf-8")):
            sys.exit(
                f"[!] Replacement too long ({len(new)} > {len(old)}): "
                f"{old!r} -> {new!r}\n"
                f"    Use a shorter server address or omit the port for port 80."
            )

    print(f"\n[*] Patching for server {ip}:{hp} (gRPC host={grpc_host}, client port from config)")
    print(f"    web URL   = {web_url}")
    print(f"    gRPC host = {grpc_host} (ensure server listens on 443 or patch client port)")

    print(f"\n[1] Patching global-metadata.dat string literals ...")
    n = patch_metadata_strings(meta, replacements)
    print(f"    {n}/{len(replacements)} strings patched")

    print(f"\n[2] Patching libil2cpp.so (SSL bypass + encryption passthrough) ...")
    n2 = patch_libil2cpp(so)
    print(f"    {n2}/{len(IL2CPP_PATCHES)} methods patched")

    print(f"\n[3] Patching AndroidManifest.xml ...")
    patch_manifest(manifest)

    print(f"\n[4] Creating network_security_config.xml ...")
    create_network_security_config(res_xml)

    print(f"\n[+] Done. Rebuild with:")
    print(f"    apktool b {apk} -o client/patched.apk")
    print(f"    apksigner sign --ks client/debug.keystore --ks-pass pass:android {apk.replace('patched/', '')}patched.apk")


if __name__ == "__main__":
    main()
