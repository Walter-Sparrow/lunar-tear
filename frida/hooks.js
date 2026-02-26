const SERVER_ADDRESS = "10.0.2.2";
const SERVER_PORT = 7777;
const HTTP_PORT = 8080;

let libil2cpp;

function awaitLibil2cpp(callback) {
    if (globalThis._hooksInstalled) return;
    try {
        libil2cpp = Process.getModuleByName("libil2cpp.so").base;
        globalThis._hooksInstalled = true;
        console.log("[*] libil2cpp.so loaded at:", libil2cpp);
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
        onEnter(args) {
            console.log("[Title] >>> OnTermOfService");
            globalThis._titleInstance = args[0];
            console.log(`[Title] _titleInstance captured: ${args[0]}`);
        }
    });
    hook("Title.OnFirstDownload", 0x30A9350, {
        onEnter(args) { console.log("[Title] >>> OnFirstDownload"); }
    });
    // OnFirstDownload.MoveNext: NO HOOK (MoveNext breaks completion chain)
    console.log("[*] OnFirstDownload.MoveNext: no hook (completion safety)");
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
    // Title.OnRegistUserName — RVA: 0x30A97BC
    // Interceptor.replace + NativeCallback: write FSM fields from JS, return completed UniTask.
    // Original code can't run — needs UI assets for dialog that aren't available.
    (function() {
        Interceptor.replace(libil2cpp.add(0x30A97BC), new NativeCallback(function(self, userdata, ct) {
            console.log("[OnRegistUserName] replaced, self=" + self);
            globalThis._titleInstance = self;
            try {
                self.add(0x3A).writeU8(1);    // _doUpdateEvent = true
                self.add(0x3C).writeS32(10);  // _requestUpdateEvent = CheckResolutionSetting
            } catch(e) { console.log("[OnRegistUserName] error: " + e); }
            return ptr(0);
        }, 'pointer', ['pointer', 'pointer', 'pointer']));
        console.log("[*] Replace OnRegistUserName -> write event=10 + return completed");
    })();

    // DialogHelper.ShowDialogEnterUserName — RVA: 0x304939C
    // Patch: return completed UniTask<bool>(true) — skip UI dialog
    (function() {
        const addr = libil2cpp.add(0x304939C);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch ShowDialogEnterUserName -> instant true");
    })();

    // DialogHelper.ShowDialogGraphicQualitySetting — RVA: 0x304946C
    // Patch: return completed UniTask<bool>(true) — skip UI dialog
    (function() {
        const addr = libil2cpp.add(0x304946C);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch ShowDialogGraphicQualitySetting -> instant true");
    })();

    // Title.OnGraphicQualitySetting — RVA: 0x30A9958
    // Interceptor.replace + NativeCallback: write FSM fields from JS, return completed UniTask.
    (function() {
        Interceptor.replace(libil2cpp.add(0x30A9958), new NativeCallback(function(self, userdata, ct) {
            console.log("[OnGraphicQualitySetting] replaced, self=" + self);
            try {
                self.add(0x3A).writeU8(1);    // _doUpdateEvent = true
                self.add(0x3C).writeS32(11);  // _requestUpdateEvent = Completion
            } catch(e) { console.log("[OnGraphicQualitySetting] error: " + e); }
            return ptr(0);
        }, 'pointer', ['pointer', 'pointer', 'pointer']));
        console.log("[*] Replace OnGraphicQualitySetting -> write event=11 + return completed");
    })();
    // Title.OnFinish — RVA: 0x30A92B4
    // NO HOOK. Let it run naturally:
    // 1. Calls CheckBeforeGamePlayAsync → server responds OK
    // 2. Returns UniTask → transition's Begin() completes
    // 3. Begin() calls OnCompleteTransition delegate → sets IsCompleted=true
    // 4. Whoever polls IsCompleted detects completion → scene transition
    //
    // IMPORTANT: OnComplete (0x30A9260) = RequestUpdate(Completion=11), NOT a finalizer!
    // It's a tail call to FSM.RequestUpdate. Do NOT call it after OnFinish.
    console.log("[*] OnFinish: no hook (natural flow + server GamePlayService)");

    // ---- QUEST BATTLE BYPASS ----
    // CalculatorQuest.StartQuest — RVA: 0x27276A4
    // Static async method: UniTask StartQuest(int questId, DifficultyType difficulty, CancellationToken ct, bool adjoiningLand, bool isRetry)
    // This method loads battle assets, runs the battle scene, shows results.
    // We don't have the asset bundles, so replace with a no-op that returns completed UniTask.
    (function() {
        Interceptor.replace(libil2cpp.add(0x27276A4), new NativeCallback(function(questId, difficulty, ct, adjoiningLand, isRetry) {
            console.log(`[QuestBattle] StartQuest BYPASSED: questId=${questId} difficulty=${difficulty} adjoiningLand=${adjoiningLand} isRetry=${isRetry}`);
            return ptr(0); // UniTask.source = null → completed immediately
        }, 'pointer', ['int', 'int', 'pointer', 'int', 'int']));
        console.log("[*] Replace CalculatorQuest.StartQuest -> instant completed UniTask (skip battle)");
    })();

    // DialogHelper.ShowDialogQuestRetry — RVA: 0x304953C
    // Returns UniTask<bool> — true=retry, false=retire.
    // Patch to return false (retire) so flow continues instead of looping.
    (function() {
        const addr = libil2cpp.add(0x304953C);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x00, 0x00, 0x80, 0xd2, // mov x0, #0 (result = false)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0 (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] Patch ShowDialogQuestRetry -> instant false (retire)");
    })();

    // SceneManager.LoadSceneAsyncNameIndexInternal — RVA: 0x4D4A45C
    hook("SceneManager.LoadSceneAsync", 0x4D4A45C, {
        onEnter(args) {
            try {
                const sceneName = args[0].isNull() ? "<null>" : readStr(args[0]);
                const sceneIndex = args[1].toInt32();
                console.log(`[SceneManager] LoadSceneAsync: name="${sceneName}" index=${sceneIndex}`);
            } catch(e) { console.log("[SceneManager] LoadSceneAsync called (error reading args: " + e + ")"); }
        }
    });

    // ---- ASSET LOADING TRACING ----
    // OctoAssetBundleLoader.LoadFromCacheOrDownload — RVA: 0x4BFBC90
    hook("OctoABLoader.LoadFromCacheOrDownload", 0x4BFBC90, {
        onEnter(args) {
            try {
                const bundleName = readStr(args[1]);
                console.log(`[OctoAB] LoadFromCacheOrDownload: "${bundleName}"`);
            } catch(e) { console.log("[OctoAB] LoadFromCacheOrDownload called"); }
        }
    });
    // OctoAssetBundleLoader.LoadFromFileAsync — RVA: 0x4BFC600
    hook("OctoABLoader.LoadFromFileAsync", 0x4BFC600, {
        onEnter(args) {
            try {
                const bundleName = readStr(args[1]);
                console.log(`[OctoAB] LoadFromFileAsync: "${bundleName}"`);
            } catch(e) { console.log("[OctoAB] LoadFromFileAsync called"); }
        }
    });
    // OctoAssetBundleLoader.DownloadToMemory — RVA: 0x4BFC2E8
    hook("OctoABLoader.DownloadToMemory", 0x4BFC2E8, {
        onEnter(args) {
            try {
                const bundleName = readStr(args[1]);
                console.log(`[OctoAB] DownloadToMemory: "${bundleName}"`);
            } catch(e) { console.log("[OctoAB] DownloadToMemory called"); }
        }
    });
    // AssetBundle.LoadFromFileAsync — RVA: 0x4D55B8C (Unity built-in)
    hook("AssetBundle.LoadFromFileAsync", 0x4D55B8C, {
        onEnter(args) {
            try {
                const path = readStr(args[0]);
                console.log(`[AssetBundle] LoadFromFileAsync: "${path}"`);
            } catch(e) { console.log("[AssetBundle] LoadFromFileAsync called"); }
        }
    });

    // ---- FSM STATE POLLER ----
    // Periodic read of Title FSM fields to track progression without hooking MoveNext.
    // Layout: CurrentState@0x10, _firstTime@0x38, _inUpdate@0x39,
    //         _doUpdateEvent@0x3A, _requestUpdateEvent@0x3C, IsCompleted@0x51
    (function() {
        const TITLE_STATES = {0:"Init",1:"InitNetwork",2:"TermOfService",3:"MaintenanceCheck",
            4:"SyncMasterData",5:"FirstDownload",6:"AgeVerification",7:"Login",
            8:"SyncUserData",9:"RegistUserName",10:"GraphicQualitySetting",11:"Finish"};
        let lastLog = "";
        let pollCount = 0;
        globalThis._fsmPoller = setInterval(function() {
            try {
                const ti = globalThis._titleInstance;
                if (!ti) return;
                const cs = ti.add(0x10).readS32();
                const ns = ti.add(0x14).readS32();
                const ft = ti.add(0x38).readU8();
                const iu = ti.add(0x39).readU8();
                const due = ti.add(0x3A).readU8();
                const rue = ti.add(0x3C).readS32();
                const ic = ti.add(0x51).readU8();
                const log = `cs=${cs}(${TITLE_STATES[cs]||"?"}) ns=${ns} ft=${ft} iu=${iu} due=${due} rue=${rue} ic=${ic}`;
                pollCount++;
                if (log !== lastLog || pollCount % 10 === 0) {
                    console.log("[FSM-POLL] " + log + (log === lastLog ? " (unchanged #" + pollCount + ")" : ""));
                    lastLog = log;
                }
            } catch(e) { console.log("[FSM-POLL] error: " + e); }
        }, 2000);
        console.log("[*] FSM state poller installed (2s interval)");
    })();

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
    hook("OctoManager.StartDbUpdate", 0x4C041B8, {
        onEnter(args) {
            console.log(`[OctoManager] StartDbUpdate called naturally (callback=${args[0]} reset=${args[1]})`);
            if (!globalThis._octoAesBypassed) {
                globalThis._octoAesBypassed = true;
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

    // HandleNet.DecryptMasterData — let it run naturally!
    // Server now serves REAL encrypted database.bin.e from archive.org.
    // The game's built-in AES decryption will handle it.
    hook("HandleNet.DecryptMasterData", 0x2775A0C, {
        onEnter(args) { console.log("[Crypto] DecryptMasterData called (NATURAL)"); },
        onLeave(retval) {
            console.log("[Crypto] DecryptMasterData returned: " + retval);
            try {
                const byteArr = retval instanceof NativePointer ? retval : ptr(retval);
                const len = byteArr.add(0x18).readU32();
                const dataStart = byteArr.add(0x20);
                console.log(`[MasterData] Decrypted byte[] length: ${len}`);
                if (len > 0) {
                    const mapByte = dataStart.readU8();
                    let tableCount = 0;
                    if (mapByte >= 0x80 && mapByte <= 0x8f) {
                        tableCount = mapByte & 0x0f;
                    } else if (mapByte === 0xde) {
                        tableCount = (dataStart.add(1).readU8() << 8) | dataStart.add(2).readU8();
                    } else if (mapByte === 0xdf) {
                        tableCount = dataStart.add(1).readU32();
                    }
                    console.log(`[MasterData] Table count: ${tableCount} (map byte: 0x${mapByte.toString(16)})`);
                }
            } catch(e) { console.log("[MasterData] Parse error: " + e); }
        }
    });

    // MemoryDatabaseBase.ctor — NO HOOK (Interceptor.attach on ctors may corrupt completion chains)
    console.log("[*] MemoryDatabaseBase.ctor: NO HOOK (safety)");

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
        onEnter(args) {
            this.self = args[0];
            console.log("[DarkClient] Constructor called!");
        },
        onLeave(retval) {
            try {
                const self = this.self;
                const oldTicks = self.add(0x30).readS64();
                const FIVE_MIN_TICKS = new Int64(5 * 60 * 10000000);
                self.add(0x30).writeS64(FIVE_MIN_TICKS);
                const oldSec = oldTicks.toNumber() / 10000000;
                console.log(`[DarkClient] deadline extended: ${oldSec.toFixed(1)}s -> 300s`);
            } catch(e) { console.log("[DarkClient] deadline patch error: " + e); }
        }
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
    // NO HOOK — Interceptor.attach on MoveNext corrupts async completion chain,
    // preventing OnTermOfService.MoveNext from being resumed after sync finishes.
    console.log("[*] SyncMasterDataAndUserData.MoveNext: no hook (completion safety)");

    // Title.<SyncMasterData>d__6.MoveNext — RVA: 0x28FEA2C
    // NO HOOK — same reason as above.
    console.log("[*] SyncMasterData.MoveNext: no hook (completion safety)");

    // Title.<SyncUserData>d__7.MoveNext — RVA: 0x28FF204
    // NO HOOK — same reason as above.
    console.log("[*] SyncUserData.MoveNext: no hook (completion safety)");

    // FSM.DoUpdate.MoveNext (0x423B594) — NO HOOK.
    // Interceptor.attach on MoveNext corrupts async state machine jump tables,
    // preventing OnCompleteTransition callback from firing.
    console.log("[*] FSM.DoUpdate.MoveNext: no hook (jump table safety)");

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

    // FSM.UpdateEvent.MoveNext (0x423C018) — NO HOOK.
    // Interceptor.attach on MoveNext corrupts jump tables,
    // breaking OnCompleteTransition dispatch in the last transition.
    console.log("[*] FSM.UpdateEvent.MoveNext: no hook (jump table safety)");

    // FSM.RequestUpdate hook REMOVED — BL patches call this directly,
    // Interceptor trampoline crashes with SIGSEGV (x8/MethodInfo=null for shared generic).
    // Log RequestUpdate via JS NativeFunction wrapper instead:
    const _RequestUpdate = new NativeFunction(libil2cpp.add(0x423D24C), 'void', ['pointer', 'int32']);
    globalThis._RequestUpdate = _RequestUpdate;

    // Title.OnComplete (0x30A9260) — NO HOOK (corrupts function).
    console.log("[*] Title.OnComplete: no hook");

    // Gameplay.CreateTitleAsync — RVA: 0x274A8D4
    hook("Gameplay.CreateTitleAsync", 0x274A8D4, {
        onEnter(args) {
            console.log("[Gameplay] CreateTitleAsync called, self=" + args[0]);
            globalThis._gameplayInstance = args[0];
        }
    });

    // Gameplay.DisposeTitle — RVA: 0x274A77C
    hook("Gameplay.DisposeTitle", 0x274A77C, {
        onEnter(args) { console.log("[Gameplay] DisposeTitle called"); }
    });

    // UniTask.WaitUntil — RVA: 0x... (static method)
    // Gameplay.<CreateTitleAsync>d__415.MoveNext — RVA: 0x289588C
    // NO HOOK — MoveNext hooks corrupt async state machine jump tables.
    console.log("[*] Gameplay.CreateTitleAsync.MoveNext: no hook (safety)");

    hook("Generator.SetupApiSystem", 0x2E96DBC, {
        onEnter(args) { console.log("[Generator] SetupApiSystem!"); }
    });

    hook("Generator.OnEntrypoint", 0x2E966A8, {
        onEnter(args) { console.log("[Generator] OnEntrypoint!"); }
    });

    // Dark.Kernel.StateMachine.SetupStateMachine — RVA: 0x2AA440C
    // Confirmed: creates HandleNet + Gameplay FSMs at startup.
    console.log("[*] StateMachine.SetupStateMachine: no hook (confirmed working)");

    // Gameplay.Generate — RVA: 0x274DCB8
    // Confirmed: called at startup, creates Gameplay FSM.
    console.log("[*] Gameplay.Generate: no hook (confirmed working)");

    // Title.Generate — RVA: 0x30A9DC0
    // Confirmed: called standalone (not via Gameplay.CreateTitleAsync).
    console.log("[*] Title.Generate: no hook (confirmed working)");

    // FSM.Setup (shared generic) — RVA: 0x423D048
    // DANGER: Interceptor.attach on shared generic corrupts MethodInfo* for
    // some instantiations. Story FSM crashed (SIGABRT) when this was hooked.
    console.log("[*] FSM.Setup: NO HOOK (shared generic, crashes Story FSM)");

    // ---- GAMEPLAY + STORY FSM DIAGNOSTIC HOOKS ----
    hook("Gameplay.StartGameplayStateMachine", 0x274E478, {
        onEnter(args) { console.log("[Gameplay] StartGameplayStateMachine called!"); }
    });

    hook("Gameplay.GetFirstGameplayEvent", 0x274E780, {
        onLeave(retval) {
            const EVENTS = {0:"Unknown",1:"RunDevelopmentMenu",2:"RunApplication",3:"LockApplication",
                4:"StartGameplay",5:"StartMainStory",6:"StartDevActorViewer",7:"StartDevUIViewer",8:"StartDevMinigame"};
            const ev = retval.toInt32();
            console.log(`[Gameplay] GetFirstGameplayEvent -> ${ev} (${EVENTS[ev]||"?"})`);
        }
    });

    hook("Gameplay.OnRunApplicationAsync", 0x274E634, {
        onEnter(args) { console.log("[Gameplay] OnRunApplicationAsync called! self=" + args[0]); }
    });

    hook("Gameplay.OnTitleAsync", 0x274E788, {
        onEnter(args) { console.log("[Gameplay] OnTitleAsync called! self=" + args[0]); }
    });

    hook("Gameplay.OnMainStoryAsync", 0x274E4D4, {
        onEnter(args) { console.log("[Gameplay] OnMainStoryAsync called! self=" + args[0]); }
    });

    hook("Gameplay.RunTitle", 0x274B29C, {
        onEnter(args) { console.log("[Gameplay] RunTitle called! self=" + args[0]); }
    });

    // Story.Generate — RVA: 0x2788E28
    // NO HOOK — Generate calls FSM.Setup internally.
    // ANY Interceptor.attach in Generate→FSM.Setup chain corrupts MethodInfo* register.
    console.log("[*] Story.Generate: NO HOOK (calls FSM.Setup internally)");

    // Story.InternalInitialize — RVA: 0x2788FD4
    // NO HOOK — Interceptor.attach corrupts MethodInfo* register in FSM.Setup caller
    console.log("[*] Story.InternalInitialize: NO HOOK (corrupts FSM.Setup)");

    // Story.SetupTransitions — RVA: 0x2789000
    console.log("[*] Story.SetupTransitions: NO HOOK (corrupts FSM.Setup)");

    // Story.ApplyFirstScene — RVA: 0x2785888
    // NO HOOK — Interceptor.attach corrupts MethodInfo* (same as other Story methods)
    console.log("[*] Story.ApplyFirstScene: NO HOOK (MethodInfo* corruption confirmed)");

    // Story.ApplyNewestScene — RVA: 0x27858E8
    // NO HOOK — same reason
    console.log("[*] Story.ApplyNewestScene: NO HOOK (MethodInfo* corruption confirmed)");

    // Debug.LogError(object) — RVA: 0x48D5108
    hook("Debug.LogError", 0x48D5108, {
        onEnter(args) {
            try {
                if (!args[0].isNull()) {
                    const klass = args[0].readPointer();
                    const toStringMI = klass.add(0x60).readPointer();
                    if (!toStringMI.isNull()) {
                        const toStringFn = new NativeFunction(toStringMI, 'pointer', ['pointer']);
                        const str = readStr(toStringFn(args[0]));
                        console.log("[Unity-ERROR] " + str);
                    }
                }
            } catch(e) { console.log("[Unity-ERROR] (unreadable)"); }
        }
    });

    // Debug.LogException(Exception) — RVA: 0x48CBA38
    hook("Debug.LogException", 0x48CBA38, {
        onEnter(args) {
            try {
                if (!args[0].isNull()) {
                    console.log("[Unity-EXCEPTION] " + args[0]);
                }
            } catch(e) { console.log("[Unity-EXCEPTION] (unreadable)"); }
        }
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

    // --- DIAGNOSTIC HOOKS: trace OnFirstDownload sub-steps ---

    hook("Title.RequestSynchronousDatabaseAsync", 0x30A8678, {
        onEnter() { console.log("[DIAG] RequestSynchronousDatabaseAsync CALLED"); },
        onLeave(retval) { console.log("[DIAG] RequestSynchronousDatabaseAsync returned: " + retval); }
    });

    hook("Title.SyncMasterDataAndUserData", 0x30A8760, {
        onEnter() { console.log("[DIAG] SyncMasterDataAndUserData CALLED"); },
        onLeave(retval) { console.log("[DIAG] SyncMasterDataAndUserData returned: " + retval); }
    });

    hook("Title.SyncMasterData", 0x30A884C, {
        onEnter() { console.log("[DIAG] SyncMasterData CALLED"); },
        onLeave(retval) { console.log("[DIAG] SyncMasterData returned: " + retval); }
    });

    // Title.SyncUserData — RVA: 0x30A8940
    // RE-ENABLED: let it run naturally now that master data has 607 real tables.
    // SyncUserData: run naturally (starts data loading pipeline), but force the
    // return value to completed UniTask<bool>(true) so FSM doesn't stall.
    // The Task->UniTask async bridge is broken, but the data pipeline itself works
    // (BuildDB + success continuation fire). By overriding x0/x1 in onLeave,
    // the caller sees a completed UniTask while data loads in background.
    hook("Title.SyncUserData", 0x30A8940, {
        onEnter() {
            console.log("[DIAG] SyncUserData CALLED (hybrid: real load + forced completion)");
        },
        onLeave(retval) {
            retval.replace(ptr(1));       // x0 = 1 (result = true)
            this.context.x1 = ptr(0);     // x1 = null (source = null → completed)
            console.log("[DIAG] SyncUserData: forced return completed(true), data loading in background");
        }
    });

    // Title.SyncPurchase — RVA: 0x30A8A10
    // BYPASS: return completed UniTask<bool>(true) instantly.
    // Unity IAP not available in emulator — skip purchase sync.
    (function() {
        const addr = libil2cpp.add(0x30A8A10);
        Memory.patchCode(addr, 12, code => {
            code.writeByteArray([
                0x20, 0x00, 0x80, 0xd2, // mov x0, #1   (result = true)
                0x01, 0x00, 0x80, 0xd2, // mov x1, #0   (source = null → completed)
                0xc0, 0x03, 0x5f, 0xd6  // ret
            ]);
        });
        console.log("[*] BYPASS SyncPurchase -> instant true");
    })();

    hook("Title.GetFirstDownloadSizeAsync", 0x30A93FC, {
        onEnter() { console.log("[DIAG] GetFirstDownloadSizeAsync CALLED"); },
        onLeave(retval) { console.log("[DIAG] GetFirstDownloadSizeAsync returned: " + retval); }
    });

    // --- DEEPER DIAGNOSTICS: UserDataGet processing ---
    // Previous run confirmed: BuildDB + b__11_1 (success continuation) fire.
    // But HandleSuccess.Invoke and FSM advancement don't happen.
    // Removing all hooks on the async pipeline to eliminate any Interceptor corruption.
    // Only keep safe read-only diagnostics.
    console.log("[*] UserDataGet pipeline: NO HOOKS (confirmed: BuildDB + success continuation fire, but FSM stalls)");

    // Hook il2cpp_raise_exception to catch ALL managed exceptions
    // Note: Module.findExportByName (static) throws "TypeError: not a function" in Frida 17.x.
    // Use instance method Process.getModuleByName().findExportByName() instead.
    try {
        console.log("[*] Looking for il2cpp_raise_exception...");
        const il2cppMod = Process.getModuleByName("libil2cpp.so");
        const raiseEx = il2cppMod.findExportByName("il2cpp_raise_exception");
        const stringCharsAddr = il2cppMod.findExportByName("il2cpp_string_chars");
        console.log("[*] il2cpp_raise_exception=" + raiseEx + " il2cpp_string_chars=" + stringCharsAddr);
        let getChars = null;
        try {
            if (stringCharsAddr) getChars = new NativeFunction(stringCharsAddr, 'pointer', ['pointer']);
        } catch(nfErr) {
            console.log("[*] NativeFunction(il2cpp_string_chars) failed: " + nfErr);
        }
        if (raiseEx && !raiseEx.isNull()) {
            // Read the first 16 bytes to see what the stub looks like
            console.log("[*] il2cpp_raise_exception bytes: " +
                Array.from(new Uint8Array(raiseEx.readByteArray(16)))
                    .map(b => b.toString(16).padStart(2, '0')).join(' '));

            // Try approach 1: follow any branch stub to find the real function
            let realTarget = raiseEx;
            const firstInsn = raiseEx.readU32();
            // Check for B (unconditional branch) instruction: 000101xx
            if ((firstInsn & 0xFC000000) === 0x14000000) {
                const imm26 = firstInsn & 0x03FFFFFF;
                const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
                realTarget = raiseEx.add(offset);
                console.log("[*] il2cpp_raise_exception is a B stub -> real target at " + realTarget);
            }

            // Try Interceptor.attach on the real target first
            let attached = false;
            try {
                Interceptor.attach(realTarget, {
                    onEnter(args) {
                        try {
                            const exObj = args[0];
                            if (exObj.isNull()) return;
                            const klass = exObj.readPointer();
                            const namePtr = klass.add(0x10).readPointer();
                            const className = namePtr.readCString();
                            let msg = "";
                            try {
                                if (getChars) {
                                    const msgField = exObj.add(0x18).readPointer();
                                    if (!msgField.isNull()) {
                                        msg = getChars(msgField).readUtf16String();
                                    }
                                }
                            } catch(e2) {}
                            console.log("[EXCEPTION] " + className + ": " + msg);
                            console.log("[EXCEPTION] stack: " +
                                Thread.backtrace(this.context, Backtracer.ACCURATE)
                                    .map(DebugSymbol.fromAddress).join('\n'));
                        } catch(e) {
                            console.log("[EXCEPTION] (couldn't read: " + e + ")");
                        }
                    }
                });
                attached = true;
                console.log("[*] Hook il2cpp_raise_exception OK (attach on real target)");
            } catch(e1) {
                console.log("[*] attach on real target failed: " + e1 + ", trying replace...");
            }

            // Approach 2: Interceptor.replace with NativeCallback
            if (!attached) {
                try {
                    const original = new NativeFunction(realTarget, 'void', ['pointer']);
                    Interceptor.replace(raiseEx, new NativeCallback(function(exObj) {
                        try {
                            if (!exObj.isNull()) {
                                const klass = exObj.readPointer();
                                const namePtr = klass.add(0x10).readPointer();
                                const className = namePtr.readCString();
                                let msg = "";
                                try {
                                    if (getChars) {
                                        const msgField = exObj.add(0x18).readPointer();
                                        if (!msgField.isNull()) {
                                            msg = getChars(msgField).readUtf16String();
                                        }
                                    }
                                } catch(e2) {}
                                console.log("[EXCEPTION] " + className + ": " + msg);
                            }
                        } catch(e) {
                            console.log("[EXCEPTION] (couldn't read: " + e + ")");
                        }
                        original(exObj);
                    }, 'void', ['pointer']));
                    console.log("[*] Hook il2cpp_raise_exception OK (replace)");
                } catch(e2) {
                    console.log("[!] il2cpp_raise_exception replace also failed: " + e2);
                }
            }
        } else {
            console.log("[!] il2cpp_raise_exception not found");
        }
    } catch(e) {
        console.log("[!] il2cpp_raise_exception hook failed: " + e);
    }

    // SyncMasterDataAndUserData — natural flow
    console.log("[*] SyncMasterDataAndUserData: NO BYPASS (natural flow)");

    // PATCH1 REMOVED — sync now works naturally with URL/DNS hooks.
    // tbnz at 0x28fab60 left original: natural flow checks sync result.
    console.log("[*] PATCH1: REMOVED (natural sync flow)");

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

    // ---- ABORT/RAISE TRAP ----
    // Catch native abort() to get full backtrace before crash
    setTimeout(() => {
        try {
            const libc = Process.getModuleByName("libc.so");

            const il2cppBase = Process.getModuleByName("libil2cpp.so").base;
            const il2cppSize = Process.getModuleByName("libil2cpp.so").size;

            function rvaOf(addr) {
                const off = addr.sub(il2cppBase);
                if (off.compare(ptr(0)) >= 0 && off.compare(ptr(il2cppSize)) < 0)
                    return "il2cpp+0x" + off.toString(16);
                return DebugSymbol.fromAddress(addr).toString();
            }

            const abortAddr = libc.findExportByName("abort");
            if (abortAddr) {
                Interceptor.replace(abortAddr, new NativeCallback(function() {
                    // Prevent actual abort — gather info first
                    const retAddr = this.context.lr;
                    const sp = this.context.sp;
                    let stackPtrs = [];
                    try {
                        for (let i = 0; i < 64; i++) {
                            const val = sp.add(i * 8).readPointer();
                            const off = val.sub(il2cppBase);
                            if (off.compare(ptr(0)) >= 0 && off.compare(ptr(il2cppSize)) < 0) {
                                stackPtrs.push("+" + (i*8) + "=" + rvaOf(val));
                            }
                        }
                    } catch(e) {}
                    console.log("[ABORT-TRAP] LR=" + rvaOf(retAddr) + " stack_il2cpp=[" + stackPtrs.join(", ") + "]");

                    // Now actually abort
                    const _raise = new NativeFunction(libc.findExportByName("raise"), 'int', ['int']);
                    _raise(6);
                }, 'void', []));
                console.log("[*] abort() REPLACED (trap + continue)");
            }
        } catch(e) {
            console.log("[!] abort/raise hook error: " + e);
        }
    }, 500);

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
