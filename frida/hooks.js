const SERVER_ADDRESS = "10.0.2.2";
const SERVER_PORT = 7777;
const HTTP_PORT = 8080;

let libil2cpp;
let hooksInstalled = false;

function awaitLibil2cpp(callback) {
    if (hooksInstalled) return;
    try {
        libil2cpp = Process.getModuleByName("libil2cpp.so").base;
        console.log("[*] libil2cpp.so loaded at:", libil2cpp);
        hooksInstalled = true;
        callback();
    } catch (error) {
        setTimeout(() => awaitLibil2cpp(callback), 100);
    }
}

function readStr(addr) {
    if (addr.isNull()) return "<null>";
    try {
        return addr.add(0x14).readUtf16String();
    } catch (e) {
        return "<err>";
    }
}

function writeStr(addr, text) {
    addr.add(0x10).writeInt(text.length);
    addr.add(0x14).writeUtf16String(text);
}

function hook(name, offset, callbacks) {
    const ptr = libil2cpp.add(offset);
    Interceptor.attach(ptr, callbacks);
    console.log(`[*] Hook ${name}`);
}

function hookReplace(name, offset, retType, argTypes, impl) {
    const ptr = libil2cpp.add(offset);
    try {
        Interceptor.replace(ptr, new NativeCallback(impl, retType, argTypes));
        console.log(`[*] Replace ${name}`);
    } catch (e) {
        console.log(`[!] Failed to replace ${name}: ${e}`);
    }
}

awaitLibil2cpp(() => {

    // ---- TITLE FSM STATE TRACING ----

    hook("Title.OnFirstStep", 0x30A9578, {
        onEnter(args) { console.log("[Title] >>> OnFirstStep"); }
    });
    hook("Title.OnPreTitle", 0x30A9714, {
        onEnter(args) { console.log("[Title] >>> OnPreTitle"); }
    });
    hook("Title.OnTitleScreen", 0x30A9D04, {
        onEnter(args) {
            console.log(`[Title] >>> OnTitleScreen`);
        }
    });
    hook("Title.OnApplicationVersion", 0x30A9128, {
        onEnter(args) { console.log("[Title] >>> OnApplicationVersion"); }
    });
    hook("Title.OnBanAccount", 0x30A91C4, {
        onEnter(args) { console.log("[Title] >>> OnBanAccount"); }
    });
    hook("Title.OnTermOfService", 0x30A9A00, {
        onEnter(args) { console.log("[Title] >>> OnTermOfService"); }
    });
    hook("Title.OnFirstDownload", 0x30A9350, {
        onEnter(args) { console.log("[Title] >>> OnFirstDownload"); }
    });
    // Title.<OnFirstDownload>d__29.MoveNext — RVA: 0x28F8278
    hook("OnFirstDownload.MoveNext", 0x28F8278, {
        onEnter(args) {
            const state = args[0].readS32();
            console.log(`[DL-FSM] MoveNext state=${state}`);
        }
    });
    // Title.InitializeAssetBundles — RVA: 0x30A94DC
    // Let it run naturally — observe HTTP/Octo requests it makes
    hook("Title.InitializeAssetBundles", 0x30A94DC, {
        onEnter() { console.log("[Title] >>> InitializeAssetBundles called (natural)"); }
    });

    // Title.LoadTextData — RVA: 0x30A9B80
    // Patched to return true — no text assets available (Octo returns empty list).
    // Game uses fallback strings. Will implement proper asset serving later.
    (function() {
        const addr = libil2cpp.add(0x30A9B80);
        Memory.patchCode(addr, 16, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true at byte 0)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6, // ret
                0x1f, 0x20, 0x03, 0xd5  // nop
            ]);
        });
        console.log("[*] Patch Title.LoadTextData -> instant true");
    })();

    // Title.OnTermOfServiceAdditionalWorldWideAsync — RVA: 0x30A9C6C
    // Let it run naturally — shows age verification and ads tracking dialogs
    hook("Title.OnTermOfServiceAdditionalWorldWideAsync", 0x30A9C6C, {
        onEnter() { console.log("[Title] >>> OnTermOfServiceAdditionalWorldWideAsync called"); }
    });

    // Title.GetFirstDownloadSizeAsync — RVA: 0x30A93FC
    hook("Title.GetFirstDownloadSizeAsync", 0x30A93FC, {
        onEnter(args) { console.log("[Title] >>> GetFirstDownloadSizeAsync"); }
    });
    // MasterDataDownloader.DownloadAsync — RVA: 0x32F0CCC
    // Let it run naturally (server returns version "0" matching fresh client's version 0).
    hook("MasterDataDownloader.DownloadAsync", 0x32F0CCC, {
        onEnter(args) { console.log("[MasterData] DownloadAsync called"); },
        onLeave(retval) { console.log("[MasterData] DownloadAsync returned: " + retval); }
    });

    // Fix crash in UniTask<bool>.get_IsCompleted (RVA: 0x4B11674).
    // UniTask<bool> struct: [source(8)|result(1)|pad(1)|token(2)|pad(4)] = 16 bytes.
    // When source is a small invalid value (e.g. 0x1 = bool true leaked), the original
    // code dereferences it and crashes. Fix: zero out bad source in-place before
    // the original code runs, so it takes the "source == null -> completed" path.
    // Fix crash in UniTask<bool>.get_IsCompleted (RVA: 0x4B11674).
    // UniTask<bool> struct with LayoutKind.Auto:
    //   offset 0: result(1) + pad(1) + token(2) + pad(4) = 8 bytes
    //   offset 8: source (IUniTaskSource<bool>*) = 8 bytes
    // When source is a small invalid value (e.g. 0x1), zero it out so the
    // original code takes the "source == null -> completed" path.
    hook("UniTask<bool>.get_IsCompleted.guard", 0x4B11674, {
        onEnter(args) {
            try {
                const structPtr = args[0];
                const source = structPtr.add(8).readPointer();
                if (!source.isNull() && source.compare(ptr(0x10000)) < 0) {
                    console.log("[IsCompleted] bad source=" + source + " -> null");
                    structPtr.add(8).writePointer(ptr(0));
                }
            } catch(e) {}
        }
    });
    hook("Title.OnRegistUserName", 0x30A97BC, {
        onEnter(args) { console.log("[Title] >>> OnRegistUserName"); }
    });
    hook("Title.OnGraphicQualitySetting", 0x30A9958, {
        onEnter(args) { console.log("[Title] >>> OnGraphicQualitySetting"); }
    });
    hook("Title.OnFinish", 0x30A92B4, {
        onEnter(args) { console.log("[Title] >>> OnFinish"); }
    });

    // ---- NETWORK INITIALIZATION ----

    hook("Title.InitializeNetworkAsync", 0x30A85DC, {
        onEnter(args) { console.log("[Title] InitializeNetworkAsync called!"); }
    });

    hook("NetworkInitializer.Initialize", 0x2C5D178, {
        onEnter(args) { console.log("[Network] NetworkInitializer.Initialize called!"); }
    });

    hook("ReviewEnvironmentComposite.UpdateEnvironment", 0x2C5D6EC, {
        onEnter(args) { console.log("[Review] UpdateEnvironment called!"); }
    });

    hook("ReviewEnvironmentComposite.GetServerConfig", 0x2C5D788, {
        onEnter(args) { console.log("[Review] GetServerConfig called!"); }
    });

    hook("UserAuthComposite.UserAuth", 0x2C5DF1C, {
        onEnter(args) { console.log("[Auth] UserAuth called!"); }
    });

    hook("UserAuthComposite.DoAuth", 0x2C5DFB8, {
        onEnter(args) { console.log("[Auth] DoAuth called!"); }
    });

    // ---- CHANNEL CREATION ----

    hook("Channel.ctor", 0x35FFA94, {
        onEnter(args) {
            try {
                const target = readStr(args[1]);
                console.log(`[Channel] NEW channel to: "${target}"`);
                writeStr(args[1], `${SERVER_ADDRESS}:${SERVER_PORT}`);
                console.log(`[Channel] Redirected to: ${SERVER_ADDRESS}:${SERVER_PORT}`);
            } catch (e) {
                console.log("[Channel] ctor hook error:", e);
            }
        }
    });

    // ---- SERVER REDIRECT ----

    hook("ChannelProvider.Setup", 0x35FF61C, {
        onEnter(args) {
            const orig = readStr(args[0]);
            console.log(`[Setup] ${orig} -> ${SERVER_ADDRESS}:${SERVER_PORT}`);
            writeStr(args[0], `${SERVER_ADDRESS}:${SERVER_PORT}`);
        }
    });

    hook("ChannelProvider.get_Channel", 0x35FF440, {
        onEnter(args) { console.log("[Channel] get_Channel called"); }
    });

    hook("ChannelProvider.Initialize", 0x35FF4D0, {
        onEnter(args) { console.log("[Channel] Initialize called"); }
    });

    hook("ServerResolver.SetConfiguration", 0x3602B8C, {
        onEnter(args) {
            try {
                const config = args[1];
                if (!config.isNull()) {
                    const serverPtr = config.add(0x10).readPointer();
                    const reviewPtr = config.add(0x18).readPointer();
                    console.log(`[SetConfig] Server=${readStr(serverPtr)} Review=${readStr(reviewPtr)}`);
                    writeStr(serverPtr, `${SERVER_ADDRESS}:${SERVER_PORT}`);
                    writeStr(reviewPtr, `${SERVER_ADDRESS}:${SERVER_PORT}`);
                }
            } catch (e) { console.log("[SetConfig] error:", e); }
        }
    });

    // ---- SSL ----

    hook("GetRootCertificateCredentials", 0x35FF8A8, {
        onEnter(args) { console.log("[SSL] GetRootCertificateCredentials called"); }
    });

    // Force insecure gRPC channel: ChannelCredentialsSafeHandle.ToNativeCredentials -> NULL
    // When NULL, Channel.ctor uses CreateInsecure instead of CreateSecure
    hookReplace("ChannelCredentials.ToNativeCredentials", 0x3603FF0,
        "pointer", ["pointer"],
        function(credentials) {
            console.log("[SSL] ToNativeCredentials -> NULL (forcing insecure channel)");
            return ptr(0);
        }
    );

    // ---- OCTO ----

    hook("DarkOctoSetupper.GetE", 0x366674C, {
        onLeave(retval) {
            const orig = readStr(retval);
            console.log(`[OctoURL] ${orig} -> http://${SERVER_ADDRESS}:${HTTP_PORT}/`);
            writeStr(retval, `http://${SERVER_ADDRESS}:${HTTP_PORT}/`);
        }
    });

    hook("DarkOctoSetupper.StartSetup", 0x3664344, {
        onEnter(args) { console.log("[OctoSetup] StartSetup called"); }
    });

    hook("DarkOctoSetupper.SetupOcto", 0x3666AAC, {
        onEnter(args) { console.log("[OctoSetup] SetupOcto called"); }
    });

    hook("DarkOctoSetupper.CreateSetting", 0x3666B9C, {
        onEnter(args) { console.log("[OctoSetup] CreateSetting called"); }
    });

    // OctoManager.Setup — RVA: 0x4C02890
    hook("OctoManager.Setup", 0x4C02890, {
        onEnter(args) { console.log("[OctoManager] Setup called"); },
        onLeave(retval) { console.log(`[OctoManager] Setup returned: ${retval}`); }
    });

    // OctoManager._Setup — RVA: 0x4C02B44
    hook("OctoManager._Setup", 0x4C02B44, {
        onEnter(args) { console.log("[OctoManager] _Setup called"); },
        onLeave(retval) { console.log(`[OctoManager] _Setup returned: ${retval}`); }
    });

    // OctoManager.StartDbUpdate — RVA: 0x4C041B8
    // Triggers DecryptAes bypass on first call (metadata guaranteed loaded by now)
    let octoAesBypassed = false;
    hook("OctoManager.StartDbUpdate", 0x4C041B8, {
        onEnter(args) {
            console.log(`[OctoManager] StartDbUpdate called naturally (callback=${args[0]} reset=${args[1]})`);
            if (!octoAesBypassed) {
                octoAesBypassed = true;
                bypassDecryptAes();
            }
        }
    });

    // ---- ENCRYPTION BYPASS ----

    hookReplace("HandleNet.Encrypt", 0x277580C, "pointer", ["pointer", "pointer"],
        function(thisArg, payload) {
            console.log("[Crypto] Encrypt bypassed");
            return payload;
        }
    );

    hookReplace("HandleNet.Decrypt", 0x277590C, "pointer", ["pointer", "pointer"],
        function(thisArg, receivedMessage) {
            console.log("[Crypto] Decrypt bypassed");
            return receivedMessage;
        }
    );

    // HandleNet.DecryptMasterData — let original decryption run (keys are in the binary)
    hook("HandleNet.DecryptMasterData", 0x2775A0C, {
        onEnter(args) { console.log("[Crypto] DecryptMasterData called"); },
        onLeave(retval) { console.log("[Crypto] DecryptMasterData done: " + retval); }
    });

    // ---- GRPC LOGGING ----

    hook("DarkClient.InvokeAsync", 0x38743FC, {
        onEnter(args) {
            try {
                const path = readStr(args[1]);
                console.log(`[gRPC] >>> ${path}`);
            } catch (e) { console.log("[gRPC] >>> (call detected)"); }
        }
    });

    hook("DarkClient.ctor", 0x27A3134, {
        onEnter(args) { console.log("[DarkClient] Constructor called!"); }
    });

    hook("ErrorHandlingInterceptor.SendAsync", 0x2E117BC, {
        onEnter(args) { console.log("[ErrorHandler] SendAsync called"); }
    });

    hook("ErrorHandlingInterceptor.ErrorHandling", 0x2E118E0, {
        onEnter(args) {
            try {
                const ex = args[1]; // RpcException
                // RpcException.status at offset 0x88 (Status struct)
                // Status contains: StatusCode (int enum) at +0x0, Detail (string) at +0x8
                const statusCode = ex.add(0x88).readS32();
                const detailPtr = ex.add(0x88 + 0x8).readPointer();
                let detail = "<null>";
                if (!detailPtr.isNull()) detail = readStr(detailPtr);
                console.log(`[ErrorHandler] RPC Exception: statusCode=${statusCode} detail="${detail}"`);
            } catch(e) {
                console.log(`[ErrorHandler] ErrorHandling (read err: ${e})`);
            }
        }
    });

    // HandleNet error methods
    hook("HandleNet.ShowStayError", 0x2776130, {
        onEnter(args) {
            try {
                console.log(`[HandleNet] ShowStayError: msg=${readStr(args[1])} code=${readStr(args[3])}`);
            } catch(e) { console.log("[HandleNet] ShowStayError called"); }
        }
    });
    hook("HandleNet.ShowMoveTitleError", 0x27762B4, {
        onEnter(args) {
            try {
                console.log(`[HandleNet] ShowMoveTitleError: msg=${readStr(args[1])} code=${readStr(args[3])}`);
            } catch(e) { console.log("[HandleNet] ShowMoveTitleError called"); }
        }
    });
    hook("HandleNet.ShowErrorDialog", 0x27771C0, {
        onEnter(args) {
            try {
                console.log(`[HandleNet] ShowErrorDialog: msg=${readStr(args[1])} arg=${readStr(args[2])} code=${readStr(args[3])}`);
            } catch(e) { console.log("[HandleNet] ShowErrorDialog called"); }
        }
    });
    hook("HandleNet.ShowAuthRetryError", 0x27764C0, {
        onEnter(args) { console.log("[HandleNet] ShowAuthRetryError called"); }
    });
    hook("HandleNet.ShowClientFatalError", 0x277655C, {
        onEnter(args) {
            try {
                console.log(`[HandleNet] ShowClientFatalError: ${readStr(args[1])}`);
            } catch(e) { console.log("[HandleNet] ShowClientFatalError called"); }
        }
    });
    hook("HandleNet.ShowMoveTitleMaintenanceError", 0x2776388, {
        onEnter(args) { console.log("[HandleNet] ShowMoveTitleMaintenanceError called"); }
    });

    // CommonDialogManager.Show — RVA: 0x30459AC
    hook("CommonDialogManager.Show", 0x30459AC, {
        onEnter(args) {
            try {
                const msgKey = readStr(args[2]);
                const title = readStr(args[3]);
                const okBtn = readStr(args[4]);
                console.log(`[DIALOG] Show: key="${msgKey}" title="${title}" ok="${okBtn}"`);
            } catch(e) { console.log("[DIALOG] Show called (read error)"); }
        }
    });

    // CommonDialogManager.ShowWithParams — RVA: 0x3045AC8
    hook("CommonDialogManager.ShowWithParams", 0x3045AC8, {
        onEnter(args) {
            try {
                const msgKey = readStr(args[2]);
                const msgArg = readStr(args[3]);
                const title = readStr(args[4]);
                console.log(`[DIALOG] ShowWithParams: key="${msgKey}" arg="${msgArg}" title="${title}"`);
            } catch(e) { console.log("[DIALOG] ShowWithParams called (read error)"); }
        }
    });

    // MessageDialogPresenter.SetData — RVA: 0x302696C
    hook("MessageDialogPresenter.SetData", 0x302696C, {
        onEnter(args) {
            try {
                const title = readStr(args[1]);
                const message = readStr(args[2]);
                const okBtn = readStr(args[3]);
                const cancelBtn = readStr(args[4]);
                console.log(`[DIALOG-SET] title="${title}" message="${message}" ok="${okBtn}" cancel="${cancelBtn}"`);
            } catch(e) { console.log("[DIALOG-SET] SetData called (read error):", e); }
        }
    });

    // ---- ASYNC STATE MACHINE TRACING ----

    // Jump table dump removed — was only needed for one-time analysis.
    // State 12 entry: RVA 0x28f9a88 → body at 0x28fab4c
    // tbnz at 0x28fab60 patched by PATCH1 below.

    // OnTermOfService.MoveNext — NO Interceptor.attach!
    // Interceptor trampoline breaks jump table dispatch inside MoveNext,
    // preventing the binary patch at 0x28fab60 from executing.
    // Rely on PATCH1 (Memory.patchCode) alone to force the TRUE path.

    // Title.<SyncMasterDataAndUserData>d__5.MoveNext — RVA: 0x28FECBC
    hook("SyncMasterDataAndUserData.MoveNext", 0x28FECBC, {
        onEnter(args) {
            try {
                this.self = args[0];
                const state = args[0].readS32();
                console.log(`[SYNC-ALL] MoveNext state=${state}`);
            } catch(e) { console.log("[SYNC-ALL] err: " + e); }
        },
        onLeave(retval) {
            try {
                const newState = this.self.readS32();
                console.log(`[SYNC-ALL] MoveNext exit -> state=${newState}`);
            } catch(e) {}
        }
    });

    // Title.<SyncMasterData>d__6.MoveNext — RVA: 0x28FEA2C
    // <>8__1 (DisplayClass6_0) at +0x28, DisplayClass6_0.isError at +0x10
    hook("SyncMasterData.MoveNext", 0x28FEA2C, {
        onEnter(args) {
            try {
                this.self = args[0];
                const state = args[0].readS32();
                console.log(`[SYNC-MASTER] MoveNext state=${state}`);
                const displayClass = args[0].add(0x28).readPointer();
                if (!displayClass.isNull()) {
                    const isError = displayClass.add(0x10).readU8();
                    console.log(`[SYNC-MASTER] isError=${isError} displayClass=${displayClass}`);
                }
            } catch(e) { console.log("[SYNC-MASTER] err: " + e); }
        },
        onLeave(retval) {
            try {
                const newState = this.self.readS32();
                console.log(`[SYNC-MASTER] MoveNext exit -> state=${newState}`);
                const displayClass = this.self.add(0x28).readPointer();
                if (!displayClass.isNull()) {
                    const isError = displayClass.add(0x10).readU8();
                    console.log(`[SYNC-MASTER] isError after=${isError}`);
                }
            } catch(e) {}
        }
    });

    // Title.<SyncUserData>d__7.MoveNext — RVA: 0x28FF204
    // <>8__1 (DisplayClass7_0) at +0x20, DisplayClass7_0.isError at +0x10
    hook("SyncUserData.MoveNext", 0x28FF204, {
        onEnter(args) {
            try {
                this.self = args[0];
                const state = args[0].readS32();
                console.log(`[SYNC-USER] MoveNext state=${state}`);
                const displayClass = args[0].add(0x20).readPointer();
                if (!displayClass.isNull()) {
                    const isError = displayClass.add(0x10).readU8();
                    console.log(`[SYNC-USER] isError=${isError} displayClass=${displayClass}`);
                }
            } catch(e) { console.log("[SYNC-USER] err: " + e); }
        },
        onLeave(retval) {
            try {
                const newState = this.self.readS32();
                console.log(`[SYNC-USER] MoveNext exit -> state=${newState}`);
                const displayClass = this.self.add(0x20).readPointer();
                if (!displayClass.isNull()) {
                    const isError = displayClass.add(0x10).readU8();
                    console.log(`[SYNC-USER] isError after=${isError}`);
                }
            } catch(e) {}
        }
    });

    // FiniteStateMachineTask<Int32Enum,Int32Enum>.DoUpdate.MoveNext — RVA: 0x423B594
    let doUpdateLogAll = false;
    hook("FSM.DoUpdate.MoveNext", 0x423B594, {
        onEnter(args) {
            try {
                const state = args[0].readS32();
                if (doUpdateLogAll || state !== 0)
                    console.log(`[FSM-DoUpdate] MoveNext state=${state} self=${args[0]}`);
            } catch(e) {}
        }
    });

    // Title.FetchTermsOfServiceVersion — RVA: 0x30A9AAC
    // Returns UniTask<int>. Method internally fetches TOS page via HTTP but always shows
    // "Failed to connect" regardless of response content (tested 200 OK with both "1" and
    // full HTML). ARM64 patch: return completed UniTask<int>(version=1).
    // UniTask<int> LayoutKind.Auto: x0=[result(4)+token(2)+pad(2)], x1=[source(8)]
    (function() {
        const addr = libil2cpp.add(0x30A9AAC);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result=1, token=0)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source=null -> completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch Title.FetchTermsOfServiceVersion -> instant complete (version=1)");
    })();

    // FiniteStateMachineTask<Int32Enum,Int32Enum>.UpdateEvent.MoveNext — RVA: 0x423C018
    hook("FSM.UpdateEvent.MoveNext", 0x423C018, {
        onEnter(args) {
            try {
                const state = args[0].readS32();
                console.log(`[FSM-UpdateEvent] MoveNext state=${state}`);
            } catch(e) {}
        }
    });

    // FiniteStateMachineTask<TitleState,TitleEvent>.RequestUpdate — RVA: 0x423D24C
    hook("FSM.RequestUpdate", 0x423D24C, {
        onEnter(args) {
            try {
                const event = args[1].toInt32();
                console.log(`[FSM] RequestUpdate event=${event} fsm=${args[0]}`);
            } catch(e) { console.log(`[FSM] RequestUpdate called`); }
        }
    });

    // Title.OnComplete — RVA: 0x30A9260
    hook("Title.OnComplete", 0x30A9260, {
        onEnter(args) { console.log("[Title] >>> OnComplete"); }
    });

    hook("Generator.SetupApiSystem", 0x2E96DBC, {
        onEnter(args) { console.log("[Generator] SetupApiSystem!"); }
    });

    hook("Generator.OnEntrypoint", 0x2E966A8, {
        onEnter(args) { console.log("[Generator] OnEntrypoint!"); }
    });

    // ---- OCTO AES DECRYPTION BYPASS ----
    // Called lazily from OctoManager.StartDbUpdate (metadata loaded by then).
    // Uses scanSync for synchronous guarantee — bypass is active before HTTP response arrives.
    function bypassDecryptAes() {
        try {
            const il2cppMod = Process.getModuleByName("libil2cpp.so");
            const pattern = "44 65 63 72 79 70 74 41 65 73 00"; // "DecryptAes\0"
            let strAddrs = [];

            const ranges = Process.enumerateRanges("r--");
            console.log(`[*] Scanning ${ranges.length} memory ranges for DecryptAes (sync)...`);
            for (const range of ranges) {
                let matches;
                try { matches = Memory.scanSync(range.base, range.size, pattern); } catch(e) { continue; }
                for (const m of matches) {
                    strAddrs.push(m.address);
                    console.log(`[*] Found 'DecryptAes' at ${m.address}`);
                }
            }
            console.log(`[*] Scan done, ${strAddrs.length} DecryptAes strings found`);
            if (strAddrs.length === 0) return;

            for (const strAddr of strAddrs) {
                const hexBytes = [];
                for (let b = 0; b < 8; b++) {
                    hexBytes.push(("0" + ((strAddr.shr(b * 8)).and(0xFF).toUInt32()).toString(16)).slice(-2));
                }
                const ptrPattern = hexBytes.join(" ");

                for (const r of Process.enumerateRanges("rw-")) {
                    let refs;
                    try { refs = Memory.scanSync(r.base, r.size, ptrPattern); } catch(e) { continue; }
                    for (const ref of refs) {
                        const miCandidate = ref.address.sub(0x10);
                        try {
                            const methodPtr = miCandidate.readPointer();
                            const rva = methodPtr.sub(il2cppMod.base);
                            if (rva.compare(ptr(0)) > 0 && rva.compare(ptr(il2cppMod.size)) < 0) {
                                console.log(`[*] DecryptAes: MethodInfo=${miCandidate} RVA=0x${rva.toString(16)}`);
                                Interceptor.replace(methodPtr, new NativeCallback(function(thisArg, bytes) {
                                    console.log("[OctoAES] DecryptAes bypassed");
                                    return bytes;
                                }, "pointer", ["pointer", "pointer"]));
                                console.log("[*] BYPASSED OctoAPI.DecryptAes!");
                            }
                        } catch(e) {}
                    }
                }
            }
        } catch(e) {
            console.log("[!] Octo AES bypass error: " + e);
        }
    }

    // PATCH: SyncMasterDataAndUserData → instant UniTask<bool>(true)
    // RVA: 0x30A8760. Replaces entire wrapper to return completed true.
    // Skips gRPC sync calls but forces TOS state 12 to get bool=TRUE.
    // Same pattern as LoadTextData and FetchTermsOfServiceVersion patches.
    (function() {
        const addr = libil2cpp.add(0x30A8760);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch SyncMasterDataAndUserData -> instant true");
    })();

    // PATCH 1 kept as belt-and-suspenders: force TRUE path at tbnz (RVA 0x28fab60)
    const patch1Addr = libil2cpp.add(0x28fab60);
    const truePath = libil2cpp.add(0x28fac74);
    Memory.patchCode(patch1Addr, 8, code => {
        const w = new Arm64Writer(code, { pc: patch1Addr });
        w.putInstruction(0x52800020); // mov w0, #1
        w.putBImm(truePath);
        w.flush();
    });
    console.log(`[*] PATCH1: tbnz→(mov w0,#1; b TRUE) at ${patch1Addr}`);

    console.log("\n[*] All IL2CPP hooks installed!");
    console.log(`[*] Target: ${SERVER_ADDRESS}:${SERVER_PORT}`);
    console.log("[*] Encryption bypass: ACTIVE\n");

    // ---- URL DOWNGRADE: https -> http for all web requests ----

    // UnityWebRequest.Get(string uri) — RVA: 0x5373BBC (static method)
    hook("UnityWebRequest.Get", 0x5373BBC, {
        onEnter(args) {
            try {
                const uri = readStr(args[0]);
                if (uri.indexOf("nierreincarnation") !== -1) {
                    const newUri = uri.replace("https://", "http://");
                    console.log(`[HTTP] GET: ${uri} -> ${newUri}`);
                    writeStr(args[0], newUri);
                } else {
                    console.log(`[HTTP] GET: ${uri}`);
                }
            } catch(e) {
                console.log("[HTTP] GET hook error:", e);
            }
        }
    });

    // UnityWebRequest..ctor(string url, string method, DownloadHandler dH, UploadHandler uH)
    // RVA: 0x5373A4C
    hook("UnityWebRequest.ctor", 0x5373A4C, {
        onEnter(args) {
            try {
                const url = readStr(args[1]);
                if (url.indexOf("nierreincarnation") !== -1) {
                    const newUrl = url.replace("https://", "http://");
                    console.log(`[HTTP] ctor: ${url} -> ${newUrl}`);
                    writeStr(args[1], newUrl);
                } else if (url.length > 3) {
                    console.log(`[HTTP] ctor: ${url}`);
                }
            } catch(e) {}
        }
    });

    // UnityWebRequest.SendWebRequest — RVA: 0x5372248
    // Catch ALL outgoing requests, log URL and downgrade https
    hook("UnityWebRequest.SendWebRequest", 0x5372248, {
        onEnter(args) {
            try {
                // args[0] = this (UnityWebRequest instance)
                const getUrlPtr = libil2cpp.add(0x53729E0);
                const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
                const urlStr = getUrl(args[0]);
                const url = readStr(urlStr);
                console.log(`[HTTP] SendWebRequest: ${url}`);
            } catch(e) {
                console.log("[HTTP] SendWebRequest hook error:", e);
            }
        }
    });

    // UnityWebRequest.Head(string uri) — RVA: 0x5373C50
    hook("UnityWebRequest.Head", 0x5373C50, {
        onEnter(args) {
            try {
                const uri = readStr(args[0]);
                if (uri.indexOf("nierreincarnation") !== -1) {
                    const newUri = uri.replace("https://", "http://");
                    console.log(`[HTTP] HEAD: ${uri} -> ${newUri}`);
                    writeStr(args[0], newUri);
                } else {
                    console.log(`[HTTP] HEAD: ${uri}`);
                }
            } catch(e) {}
        }
    });

    // UnityWebRequest.Post — RVA: 0x5373CBC
    hook("UnityWebRequest.Post", 0x5373CBC, {
        onEnter(args) {
            try {
                const uri = readStr(args[0]);
                console.log(`[HTTP] POST: ${uri}`);
                if (uri.indexOf("nierreincarnation") !== -1) {
                    writeStr(args[0], uri.replace("https://", "http://"));
                }
            } catch(e) {}
        }
    });

    // ---- DOWNLOAD HANDLER TEXT INSPECTION ----
    // DownloadHandler.get_text — RVA: 0x5371194
    hook("DownloadHandler.get_text", 0x5371194, {
        onLeave(retval) {
            try {
                if (!retval.isNull()) {
                    const text = readStr(retval);
                    console.log(`[HTTP] downloadHandler.text = "${text.substring(0, 200)}"`);
                } else {
                    console.log("[HTTP] downloadHandler.text = NULL");
                }
            } catch(e) { console.log("[HTTP] downloadHandler.text error:", e); }
        }
    });

    // ---- HTTP ERROR INSPECTION ----
    // UnityWebRequest.get_isNetworkError — RVA: 0x53727F4
    hook("UnityWebRequest.get_isNetworkError", 0x53727F4, {
        onEnter(args) { this.self = args[0]; },
        onLeave(retval) {
            try {
                const getUrlPtr = libil2cpp.add(0x53729E0);
                const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
                const urlStr = getUrl(this.self);
                const url = readStr(urlStr);
                const isErr = retval.toInt32();
                if (isErr !== 0) console.log(`[HTTP] isNetworkError=TRUE url=${url}`);
                else console.log(`[HTTP] isNetworkError=false url=${url}`);
            } catch(e) {
                if (retval.toInt32() !== 0) console.log("[HTTP] isNetworkError = TRUE!");
            }
        }
    });

    // UnityWebRequest.get_isHttpError — RVA: 0x5372834
    hook("UnityWebRequest.get_isHttpError", 0x5372834, {
        onEnter(args) { this.self = args[0]; },
        onLeave(retval) {
            try {
                const getUrlPtr = libil2cpp.add(0x53729E0);
                const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
                const urlStr = getUrl(this.self);
                const url = readStr(urlStr);
                const isErr = retval.toInt32();
                if (isErr !== 0) console.log(`[HTTP] isHttpError=TRUE url=${url}`);
                else console.log(`[HTTP] isHttpError=false url=${url}`);
            } catch(e) {
                if (retval.toInt32() !== 0) console.log("[HTTP] isHttpError = TRUE!");
            }
        }
    });

    // UnityWebRequest.get_error — RVA: 0x53725FC
    hook("UnityWebRequest.get_error", 0x53725FC, {
        onEnter(args) { this.self = args[0]; },
        onLeave(retval) {
            try {
                if (!retval.isNull()) {
                    const err = readStr(retval);
                    if (err.length > 0) {
                        const getUrlPtr = libil2cpp.add(0x53729E0);
                        const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
                        const urlStr = getUrl(this.self);
                        const url = readStr(urlStr);
                        console.log(`[HTTP] error="${err}" url=${url}`);
                    }
                }
            } catch(e) {}
        }
    });

    // UnityWebRequest.get_responseCode — RVA: 0x5372874
    hook("UnityWebRequest.get_responseCode", 0x5372874, {
        onEnter(args) { this.self = args[0]; },
        onLeave(retval) {
            try {
                const code = retval.toInt32();
                const getUrlPtr = libil2cpp.add(0x53729E0);
                const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
                const urlStr = getUrl(this.self);
                const url = readStr(urlStr);
                console.log(`[HTTP] responseCode=${code} url=${url}`);
            } catch(e) {
                console.log(`[HTTP] responseCode=${retval.toInt32()}`);
            }
        }
    });

    // ---- FORCE INTERNET REACHABILITY ----
    hookReplace("Application.get_internetReachability", 0x48CAE78,
        "int", [],
        function() {
            return 2;
        }
    );

    // TitleStubDelegator.IsValidTermOfService — RVA: 0x28FFAF8
    // Force true to trigger TOS + age verification dialogs.
    // These dialogs set consent flags that OnTermOfService needs for proper completion.
    hook("TitleStubDelegator.IsValidTermOfService", 0x28FFAF8, {
        onLeave(retval) {
            const orig = retval.toInt32();
            retval.replace(ptr(1));
            console.log(`[Delegator] IsValidTermOfService: ${orig} -> forced 1`);
        }
    });

    // ---- DNS REDIRECT via getaddrinfo hook ----
    // Redirect nierreincarnation.com hostnames to 10.0.2.2
    setTimeout(() => {
        try {
            const mod = Process.getModuleByName("libc.so");
            console.log("[*] libc.so at:", mod.base, "size:", mod.size);

            const getaddrinfoAddr = mod.findExportByName("getaddrinfo");
            console.log("[*] getaddrinfo at:", getaddrinfoAddr);

            if (getaddrinfoAddr) {
                Interceptor.attach(getaddrinfoAddr, {
                    onEnter(args) {
                        try {
                            const hostname = args[0].readCString();
                            if (hostname && hostname.indexOf("nierreincarnation") !== -1) {
                                console.log(`[DNS] REDIRECT ${hostname} -> ${SERVER_ADDRESS}`);
                                // Replace hostname with our server IP
                                args[0].writeUtf8String(SERVER_ADDRESS);
                            }
                        } catch(e) {}
                    }
                });
                console.log("[*] getaddrinfo hook installed!");
            } else {
                console.log("[!] getaddrinfo not found in libc.so");
            }
        } catch(e) {
            console.log("[!] DNS hook error:", e);
            console.log("[!] Error stack:", e.stack);
        }
    }, 1000);
});
