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
        onEnter(args) { console.log("[Title] >>> OnTitleScreen"); }
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
    // Returns UniTask (non-generic, 16 bytes): x0=0, x1=0 → source=null, token=0 → completed
    (function() {
        const addr = libil2cpp.add(0x30A94DC);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x00, 0x00, 0x80, 0xd2, // mov x0, #0
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch Title.InitializeAssetBundles -> instant complete");
    })();
    hook("Title.InitializeAssetBundles.log", 0x30A94DC, {
        onEnter() { console.log("[Title] >>> InitializeAssetBundles called"); }
    });

    // Title.LoadTextData — RVA: 0x30A9B80
    // Returns UniTask<bool> (16 bytes) in registers.
    // LayoutKind.Auto: x0 = [result(1)|pad(1)|token(2)|pad(4)], x1 = [source(8)]
    // Completed with result=true: x0=1, x1=0 (source=null)
    (function() {
        const addr = libil2cpp.add(0x30A9B80);
        Memory.patchCode(addr, 16, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true at byte 0)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null)
                0xc0, 0x03, 0x5f, 0xd6, // ret
                0x1f, 0x20, 0x03, 0xd5  // nop
            ]);
        });
        console.log("[*] Patch Title.LoadTextData -> instant complete (true)");
    })();
    hook("Title.LoadTextData.log", 0x30A9B80, {
        onEnter() { console.log("[Title] >>> LoadTextData called"); }
    });

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
    // static void StartDbUpdate(Action<DownloadError> onComplete, bool reset)
    // Bypass the actual DB update but invoke the callback with null (no error).
    // Action<T>.Invoke RVA: 0x4A3D2A8 — shared for reference-type T
    hookReplace("OctoManager.StartDbUpdate", 0x4C041B8,
        "void", ["pointer", "int"],
        function(onComplete, reset) {
            console.log("[OctoManager] StartDbUpdate BYPASSED — invoking callback with null (no error)");
            if (!onComplete.isNull()) {
                const actionInvoke = new NativeFunction(
                    libil2cpp.add(0x4A3D2A8), "void", ["pointer", "pointer", "pointer"]
                );
                setTimeout(() => {
                    try {
                        actionInvoke(onComplete, ptr(0), ptr(0));
                        console.log("[OctoManager] StartDbUpdate callback invoked OK");
                    } catch(e) {
                        console.log("[OctoManager] StartDbUpdate callback error: " + e);
                    }
                }, 100);
            }
        }
    );

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

    let tosCompleted = false;
    let capturedFsmPtr = null;
    let capturedMethodInfo = null;

    // Title.<OnTermOfService>d__40.MoveNext — RVA: 0x28F99A8
    hook("OnTermOfService.MoveNext", 0x28F99A8, {
        onEnter(args) {
            try {
                this.self = args[0];
                const state = args[0].readS32();
                console.log(`[TOS-FSM] MoveNext state=${state}`);
            } catch(e) { console.log("[TOS-FSM] MoveNext called (err: " + e + ")"); }
        },
        onLeave(retval) {
            try {
                const newState = this.self.readS32();
                console.log(`[TOS-FSM] MoveNext exit -> state=${newState}`);
                if (newState === -2 && capturedFsmPtr && capturedMethodInfo) {
                    tosCompleted = true;
                    console.log(`[TOS-FSM] COMPLETED — scheduling RequestUpdate(7) + FSM[0x3a] set`);
                    const fsmPtr = capturedFsmPtr;
                    const methodInfo = capturedMethodInfo;
                    try {
                        const before = fsmPtr.add(0x3a).readU8();
                        console.log(`[TOS-FSM] FSM[0x3a] before = ${before}`);
                    } catch(e) {}
                    setTimeout(() => {
                        try {
                            const nativeRequestUpdate = new NativeFunction(
                                libil2cpp.add(0x423D24C), "void", ["pointer", "int", "pointer"]
                            );
                            nativeRequestUpdate(fsmPtr, 7, methodInfo);
                            console.log("[TOS-FSM] RequestUpdate(7) OK!");
                            const after = fsmPtr.add(0x3a).readU8();
                            console.log(`[TOS-FSM] FSM[0x3a] after RequestUpdate = ${after}`);
                            if (after === 0) {
                                fsmPtr.add(0x3a).writeU8(1);
                                console.log("[TOS-FSM] FSM[0x3a] forced to 1");
                            }
                        } catch(e) {
                            console.log("[TOS-FSM] RequestUpdate(7) error: " + e);
                        }
                    }, 100);
                }
            } catch(e) {}
        }
    });

    // FiniteStateMachineTask<Int32Enum,Int32Enum>.DoUpdate.MoveNext — RVA: 0x423B594
    let doUpdateDumpCount = 0;
    hook("FSM.DoUpdate.MoveNext", 0x423B594, {
        onEnter(args) {
            try {
                const state = args[0].readS32();
                if (state !== 0) console.log(`[FSM-DoUpdate] MoveNext state=${state}`);
                if (state === 0 && tosCompleted && doUpdateDumpCount < 5) {
                    doUpdateDumpCount++;
                    try {
                        const self = args[0];
                        const fsmRef = self.add(0x18).readPointer();
                        const byte3a = fsmRef.add(0x3a).readU8();
                        const byte38 = fsmRef.add(0x38).readU8();
                        const field20 = self.add(0x20).readS32();
                        console.log(`[FSM-DoUpdate] state=0 post-TOS #${doUpdateDumpCount}: FSM[0x38]=${byte38} FSM[0x3a]=${byte3a} self[0x20]=${field20}`);
                    } catch(ex) { console.log(`[FSM-DoUpdate] dump err: ${ex}`); }
                }
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
                if (event === 8) {
                    capturedFsmPtr = args[0];
                    capturedMethodInfo = args[2];
                    console.log(`[FSM] Captured FSM=${capturedFsmPtr} MethodInfo=${capturedMethodInfo}`);
                }
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
    // Force return true — TOS is already accepted, skip dialog flow.
    // The dialog path breaks because our other patches (InitializeAssetBundles,
    // LoadTextData) don't actually load the resources the dialog flow needs.
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
