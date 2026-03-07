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

  hook("Title.OnFirstStep", 0x30a9578, {
    onEnter(args) {
      console.log("[Title] >>> OnFirstStep");
    },
  });
  hook("Title.OnPreTitle", 0x30a9714, {
    onEnter(args) {
      console.log("[Title] >>> OnPreTitle");
    },
  });
  hook("Title.OnTitleScreen", 0x30a9d04, {
    onEnter(args) {
      console.log(`[Title] >>> OnTitleScreen`);
    },
  });
  hook("Title.OnApplicationVersion", 0x30a9128, {
    onEnter(args) {
      console.log("[Title] >>> OnApplicationVersion");
    },
  });
  hook("Title.OnBanAccount", 0x30a91c4, {
    onEnter(args) {
      console.log("[Title] >>> OnBanAccount");
    },
  });
  hook("Title.OnTermOfService", 0x30a9a00, {
    onEnter(args) {
      console.log("[Title] >>> OnTermOfService");
      globalThis._titleInstance = args[0];
      console.log(`[Title] _titleInstance captured: ${args[0]}`);
    },
  });
  hook("Title.OnFirstDownload", 0x30a9350, {
    onEnter(args) {
      console.log("[Title] >>> OnFirstDownload");
    },
  });
  // OnFirstDownload.MoveNext: NO HOOK (MoveNext breaks completion chain)
  console.log("[*] OnFirstDownload.MoveNext: no hook (completion safety)");
  // Title.InitializeAssetBundles — RVA: 0x30A94DC
  // Let it run naturally — observe HTTP/Octo requests it makes
  hook("Title.InitializeAssetBundles", 0x30a94dc, {
    onEnter() {
      console.log("[Title] >>> InitializeAssetBundles called (natural)");
    },
  });

  // Title.LoadTextData — RVA: 0x30A9B80
  // Patched to return true — no text assets available (Octo returns empty list).
  // Game uses fallback strings. Will implement proper asset serving later.
  (function () {
    const addr = libil2cpp.add(0x30a9b80);
    Memory.patchCode(addr, 16, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result = true at byte 0)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
        0x1f,
        0x20,
        0x03,
        0xd5, // nop
      ]);
    });
    console.log("[*] Patch Title.LoadTextData -> instant true");
  })();

  // Title.OnTermOfServiceAdditionalWorldWideAsync — RVA: 0x30A9C6C
  // Let it run naturally — shows age verification and ads tracking dialogs
  hook("Title.OnTermOfServiceAdditionalWorldWideAsync", 0x30a9c6c, {
    onEnter() {
      console.log("[Title] >>> OnTermOfServiceAdditionalWorldWideAsync called");
    },
  });

  // Title.GetFirstDownloadSizeAsync — RVA: 0x30A93FC
  hook("Title.GetFirstDownloadSizeAsync", 0x30a93fc, {
    onEnter(args) {
      console.log("[Title] >>> GetFirstDownloadSizeAsync");
    },
  });
  // MasterDataDownloader.DownloadAsync — RVA: 0x32F0CCC
  // Let it run naturally (server returns version "0" matching fresh client's version 0).
  hook("MasterDataDownloader.DownloadAsync", 0x32f0ccc, {
    onEnter(args) {
      console.log("[MasterData] DownloadAsync called");
    },
    onLeave(retval) {
      console.log("[MasterData] DownloadAsync returned: " + retval);
    },
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
  hook("UniTask<bool>.get_IsCompleted.guard", 0x4b11674, {
    onEnter(args) {
      try {
        const structPtr = args[0];
        const source = structPtr.add(8).readPointer();
        if (!source.isNull() && source.compare(ptr(0x10000)) < 0) {
          console.log("[IsCompleted] bad source=" + source + " -> null");
          structPtr.add(8).writePointer(ptr(0));
        }
      } catch (e) {}
    },
  });
  // Title.OnRegistUserName — RVA: 0x30A97BC
  // Interceptor.replace + NativeCallback: write FSM fields from JS, return completed UniTask.
  // Original code can't run — needs UI assets for dialog that aren't available.
  (function () {
    Interceptor.replace(
      libil2cpp.add(0x30a97bc),
      new NativeCallback(
        function (self, userdata, ct) {
          console.log("[OnRegistUserName] replaced, self=" + self);
          globalThis._titleInstance = self;
          try {
            self.add(0x3a).writeU8(1); // _doUpdateEvent = true
            self.add(0x3c).writeS32(10); // _requestUpdateEvent = CheckResolutionSetting
          } catch (e) {
            console.log("[OnRegistUserName] error: " + e);
          }
          return ptr(0);
        },
        "pointer",
        ["pointer", "pointer", "pointer"]
      )
    );
    console.log(
      "[*] Replace OnRegistUserName -> write event=10 + return completed"
    );
  })();

  // DialogHelper.ShowDialogEnterUserName — RVA: 0x304939C
  // Patch: return completed UniTask<bool>(true) — skip UI dialog
  (function () {
    const addr = libil2cpp.add(0x304939c);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result = true)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log("[*] Patch ShowDialogEnterUserName -> instant true");
  })();

  // DialogHelper.ShowDialogGraphicQualitySetting — RVA: 0x304946C
  // Patch: return completed UniTask<bool>(true) — skip UI dialog
  (function () {
    const addr = libil2cpp.add(0x304946c);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result = true)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log("[*] Patch ShowDialogGraphicQualitySetting -> instant true");
  })();

  // Title.OnGraphicQualitySetting — RVA: 0x30A9958
  // Interceptor.replace + NativeCallback: write FSM fields from JS, return completed UniTask.
  (function () {
    Interceptor.replace(
      libil2cpp.add(0x30a9958),
      new NativeCallback(
        function (self, userdata, ct) {
          console.log("[OnGraphicQualitySetting] replaced, self=" + self);
          try {
            self.add(0x3a).writeU8(1); // _doUpdateEvent = true
            self.add(0x3c).writeS32(11); // _requestUpdateEvent = Completion
          } catch (e) {
            console.log("[OnGraphicQualitySetting] error: " + e);
          }
          return ptr(0);
        },
        "pointer",
        ["pointer", "pointer", "pointer"]
      )
    );
    console.log(
      "[*] Replace OnGraphicQualitySetting -> write event=11 + return completed"
    );
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
  (function () {
    Interceptor.replace(
      libil2cpp.add(0x27276a4),
      new NativeCallback(
        function (questId, difficulty, ct, adjoiningLand, isRetry) {
          console.log(
            `[QuestBattle] StartQuest BYPASSED: questId=${questId} difficulty=${difficulty} adjoiningLand=${adjoiningLand} isRetry=${isRetry}`
          );
          return ptr(0); // UniTask.source = null → completed immediately
        },
        "pointer",
        ["int", "int", "pointer", "int", "int"]
      )
    );
    console.log(
      "[*] Replace CalculatorQuest.StartQuest -> instant completed UniTask (skip battle)"
    );
  })();

  // DialogHelper.ShowDialogQuestRetry — RVA: 0x304953C
  // Returns UniTask<bool> — true=retry, false=retire.
  // Patch to return false (retire) so flow continues instead of looping.
  (function () {
    const addr = libil2cpp.add(0x304953c);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x00,
        0x00,
        0x80,
        0xd2, // mov x0, #0 (result = false)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0 (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log("[*] Patch ShowDialogQuestRetry -> instant false (retire)");
  })();

  // SceneManager.LoadSceneAsyncNameIndexInternal — RVA: 0x4D4A45C
  hook("SceneManager.LoadSceneAsync", 0x4d4a45c, {
    onEnter(args) {
      try {
        const sceneName = args[0].isNull() ? "<null>" : readStr(args[0]);
        const sceneIndex = args[1].toInt32();
        console.log(
          `[SceneManager] LoadSceneAsync: name="${sceneName}" index=${sceneIndex}`
        );
      } catch (e) {
        console.log(
          "[SceneManager] LoadSceneAsync called (error reading args: " + e + ")"
        );
      }
    },
  });

  // ---- ASSET LOADING TRACING ----
  // OctoAssetBundleLoader.LoadFromCacheOrDownload — RVA: 0x4BFBC90
  hook("OctoABLoader.LoadFromCacheOrDownload", 0x4bfbc90, {
    onEnter(args) {
      try {
        const bundleName = readStr(args[1]);
        console.log(`[OctoAB] LoadFromCacheOrDownload: "${bundleName}"`);
      } catch (e) {
        console.log("[OctoAB] LoadFromCacheOrDownload called");
      }
    },
  });
  // OctoAssetBundleLoader.LoadFromFileAsync — RVA: 0x4BFC600
  hook("OctoABLoader.LoadFromFileAsync", 0x4bfc600, {
    onEnter(args) {
      try {
        const bundleName = readStr(args[1]);
        console.log(`[OctoAB] LoadFromFileAsync: "${bundleName}"`);
      } catch (e) {
        console.log("[OctoAB] LoadFromFileAsync called");
      }
    },
  });
  // OctoAssetBundleLoader.DownloadToMemory — RVA: 0x4BFC2E8
  hook("OctoABLoader.DownloadToMemory", 0x4bfc2e8, {
    onEnter(args) {
      try {
        const bundleName = readStr(args[1]);
        console.log(`[OctoAB] DownloadToMemory: "${bundleName}"`);
      } catch (e) {
        console.log("[OctoAB] DownloadToMemory called");
      }
    },
  });
  // AssetBundle.LoadFromFileAsync — RVA: 0x4D55B8C (Unity built-in)
  hook("AssetBundle.LoadFromFileAsync", 0x4d55b8c, {
    onEnter(args) {
      try {
        const path = readStr(args[0]);
        console.log(`[AssetBundle] LoadFromFileAsync: "${path}"`);
      } catch (e) {
        console.log("[AssetBundle] LoadFromFileAsync called");
      }
    },
  });

  // OnMainStoryAsync.MoveNext — NO HOOK
  // MoveNext hooks corrupt async state machine jump tables.
  console.log("[*] OnMainStoryAsync.MoveNext: NO HOOK (natural flow)");

  // Story.ApplyFirstScene — RVA: 0x2785888
  // NO HOOK — Interceptor.attach on Story methods causes MethodInfo* corruption → SIGSEGV
  console.log("[*] Story.ApplyFirstScene: NO HOOK (MethodInfo* corruption)");
  console.log("[*] Story.ApplyNewestScene: NO HOOK (MethodInfo* corruption)");

  // ---- DarkUserDatabaseBuilder tracing ----
  // Append(IEnumerable<EntityIUserMainQuestMainFlowStatus>) - RVA: 0x2613948
  hook("DarkUserDatabaseBuilder.Append(MainQuestMainFlowStatus)", 0x2613948, {
    onEnter(args) {
      console.log("[DB] Append MainQuestMainFlowStatus called");
      try {
        // args[1] is IEnumerable<Dictionary<string, object>> - try to read first element
        const enumerable = args[1];
        console.log(`[DB] dataSource ptr: ${enumerable}`);
      } catch (e) {
        console.log(`[DB] error reading dataSource: ${e}`);
      }
    },
    onLeave(retval) {
      console.log(`[DB] Append returned: ${retval}`);
    },
  });

  // ---- ActivePlayer accessors (safe to hook — static methods) ----
  hook("ActivePlayerToEntityMainQuestStatus", 0x2ab491c, {
    onEnter(args) {
      console.log("[UserData] ActivePlayerToEntityMainQuestStatus called");
    },
    onLeave(retval) {
      if (retval.isNull()) {
        console.log("[UserData] ActivePlayerToEntityMainQuestStatus -> NULL");
      } else {
        try {
          const userId = retval.readS64();
          const routeId = retval.add(8).readS32();
          const curScene = retval.add(12).readS32();
          const headScene = retval.add(16).readS32();
          const isReached = retval.add(20).readU8();
          console.log(
            `[UserData] MainQuestStatus: routeId=${routeId} curScene=${curScene} headScene=${headScene} isReached=${isReached}`
          );
        } catch (e) {
          console.log(
            `[UserData] MainQuestStatus at ${retval} (read err: ${e})`
          );
        }
      }
    },
  });
  hook("ActivePlayerToEntityReplayFlowStatus", 0x2ab4ca0, {
    onEnter(args) {
      console.log("[UserData] ActivePlayerToEntityReplayFlowStatus called");
    },
    onLeave(retval) {
      console.log(`[UserData] ReplayFlowStatus -> ${retval}`);
    },
  });

  // ---- ASYNC EXCEPTION TRACING ----
  // AsyncUniTaskMethodBuilder.SetException — RVA: 0x408C594
  // Catches any exception thrown inside async UniTask methods (silent exceptions)
  hook("AsyncUniTaskMethodBuilder.SetException", 0x408c594, {
    onEnter(args) {
      try {
        const exc = args[1];
        if (exc.isNull()) {
          console.log("[ASYNC-EXC] SetException called with null");
          return;
        }
        // Il2CppObject: klass at +0, klass->name at klass+0x10 (Il2CppClass.name)
        const klass = exc.readPointer();
        const namePtr = klass.add(0x10).readPointer();
        const typeName = namePtr.readCString();
        // Try to read _message field (System.Exception._message at offset 0x18 typically)
        let msg = "";
        try {
          const msgObj = exc.add(0x18).readPointer();
          if (!msgObj.isNull()) {
            // Il2CppString: length at +0x10, chars at +0x14
            const len = msgObj.add(0x10).readS32();
            msg = msgObj.add(0x14).readUtf16String(len);
          }
        } catch (e2) {}
        console.log(`[ASYNC-EXC] SetException: ${typeName}: ${msg}`);
        console.log(
          `[ASYNC-EXC] Stack: ${Thread.backtrace(
            this.context,
            Backtracer.ACCURATE
          )
            .map(DebugSymbol.fromAddress)
            .join("\\n")}`
        );
      } catch (e) {
        console.log(
          "[ASYNC-EXC] SetException called (parse failed: " + e + ")"
        );
      }
    },
  });

  // ---- GAMEPLAY FLOW TRACING ----
  hook("Gameplay.CreateAsyncTitleEndContents", 0x274a97c, {
    onEnter(args) {
      console.log("[Gameplay] >>> CreateAsyncTitleEndContents");
    },
  });
  hook("Gameplay.OnRunApplicationAsync", 0x274e634, {
    onEnter(args) {
      globalThis._gameplayPtr = args[0];
      console.log("[Gameplay] >>> OnRunApplicationAsync self=" + args[0]);
    },
  });
  hook("Gameplay.WaitInitializedScene", 0x274e6e4, {
    onEnter(args) {
      console.log("[Gameplay] >>> WaitInitializedScene");
    },
  });
  hook("Gameplay.StartGameplayStateMachine", 0x274e478, {
    onEnter(args) {
      console.log("[Gameplay] >>> StartGameplayStateMachine");
    },
  });
  hook("Gameplay.GetFirstGameplayEvent", 0x274e780, {
    onEnter(args) {
      console.log("[Gameplay] >>> GetFirstGameplayEvent");
    },
  });
  hook("Gameplay.OnTitleAsync", 0x274e788, {
    onEnter(args) {
      console.log("[Gameplay] >>> OnTitleAsync");
    },
  });
  // BYPASS: PlayTitleFlowMovieAsync → return completed UniTask (no movie assets)
  (function () {
    const addr = libil2cpp.add(0x274e580);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x00,
        0x00,
        0x80,
        0xd2, // mov x0, #0   (source = null → completed)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log(
      "[*] PatchCode Gameplay.PlayTitleFlowMovieAsync → completed UniTask"
    );
  })();

  // ---- FSM STATE POLLER ----
  // Periodic read of Title FSM fields to track progression without hooking MoveNext.
  // Layout: CurrentState@0x10, _firstTime@0x38, _inUpdate@0x39,
  //         _doUpdateEvent@0x3A, _requestUpdateEvent@0x3C, IsCompleted@0x51
  (function () {
    const TITLE_STATES = {
      0: "Init",
      1: "InitNetwork",
      2: "TermOfService",
      3: "MaintenanceCheck",
      4: "SyncMasterData",
      5: "FirstDownload",
      6: "AgeVerification",
      7: "Login",
      8: "SyncUserData",
      9: "RegistUserName",
      10: "GraphicQualitySetting",
      11: "Finish",
    };
    let lastLog = "";
    let pollCount = 0;
    globalThis._fsmPoller = setInterval(function () {
      try {
        const ti = globalThis._titleInstance;
        if (!ti) return;
        const cs = ti.add(0x10).readS32();
        const ns = ti.add(0x14).readS32();
        const ft = ti.add(0x38).readU8();
        const iu = ti.add(0x39).readU8();
        const due = ti.add(0x3a).readU8();
        const rue = ti.add(0x3c).readS32();
        const ic = ti.add(0x51).readU8();
        const log = `cs=${cs}(${
          TITLE_STATES[cs] || "?"
        }) ns=${ns} ft=${ft} iu=${iu} due=${due} rue=${rue} ic=${ic}`;
        pollCount++;
        if (log !== lastLog || pollCount % 10 === 0) {
          console.log(
            "[FSM-POLL] " +
              log +
              (log === lastLog ? " (unchanged #" + pollCount + ")" : "")
          );
          lastLog = log;
        }
      } catch (e) {
        console.log("[FSM-POLL] error: " + e);
      }

      // Gameplay FSM poller (same timer)
      try {
        const gp = globalThis._gameplayPtr;
        if (gp) {
          const GAMEPLAY_STATES = {
            0: "Unknown",
            1: "DevelopConfig",
            2: "FirstStep",
            3: "LockApp",
            4: "MainStory",
            5: "Title",
          };
          const gcs = ptr(gp).add(0x10).readS32();
          const gns = ptr(gp).add(0x14).readS32();
          const giu = ptr(gp).add(0x39).readU8();
          const gdue = ptr(gp).add(0x3a).readU8();
          const grue = ptr(gp).add(0x3c).readS32();
          const cwsrPtr = ptr(gp).add(0x158);
          const cwsr = cwsrPtr.readU8();
          // WaitCompletionScene unblock: when in MainStory flow, ALWAYS force cwsr=1 every poll.
          // Game resets it after reading; we must keep it set until flow progresses past WaitCompletionScene.
          const inMainStoryFlow = gcs === 4 || (gcs === 5 && gns === 4);
          if (inMainStoryFlow) {
            if (cwsr === 0) {
              cwsrPtr.writeU8(1);
              if (
                !globalThis._lastCwsrForceLog ||
                pollCount - globalThis._lastCwsrForceLog > 5
              ) {
                console.log(
                  "[GP-POLL] Forced CompletedWaitSceneRequestReplace=1 (unblock WaitCompletionScene)"
                );
                globalThis._lastCwsrForceLog = pollCount;
              }
            }
          }
          const cwsrNow = ptr(gp).add(0x158).readU8();
          const gpLog = `gcs=${gcs}(${
            GAMEPLAY_STATES[gcs] || "?"
          }) gns=${gns} giu=${giu} gdue=${gdue} grue=${grue} cwsr=${cwsrNow}`;
          if (
            !globalThis._lastGpLog ||
            gpLog !== globalThis._lastGpLog ||
            pollCount % 10 === 0
          ) {
            console.log(
              "[GP-POLL] " +
                gpLog +
                (gpLog === globalThis._lastGpLog
                  ? " (unchanged #" + pollCount + ")"
                  : "")
            );
            globalThis._lastGpLog = gpLog;
          }
        }
      } catch (e2) {}
    }, 2000);
    console.log("[*] FSM state poller installed (2s interval)");

    // Fast cwsr forcer: 200ms — game resets cwsr quickly, we must keep it set
    globalThis._cwsrForcer = setInterval(function () {
      try {
        const gp = globalThis._gameplayPtr;
        if (!gp) return;
        const gcs = ptr(gp).add(0x10).readS32();
        const gns = ptr(gp).add(0x14).readS32();
        const inMainStoryFlow = gcs === 4 || (gcs === 5 && gns === 4);
        if (inMainStoryFlow) {
          ptr(gp).add(0x158).writeU8(1);
        }
      } catch (e) {}
    }, 200);
    console.log("[*] WaitCompletionScene forcer installed (200ms)");
  })();

  // ---- NETWORK INITIALIZATION ----

  hook("Title.InitializeNetworkAsync", 0x30a85dc, {
    onEnter(args) {
      console.log("[Title] InitializeNetworkAsync called!");
    },
  });

  hook("NetworkInitializer.Initialize", 0x2c5d178, {
    onEnter(args) {
      console.log("[Network] NetworkInitializer.Initialize called!");
    },
  });

  hook("ReviewEnvironmentComposite.UpdateEnvironment", 0x2c5d6ec, {
    onEnter(args) {
      console.log("[Review] UpdateEnvironment called!");
    },
  });

  hook("ReviewEnvironmentComposite.GetServerConfig", 0x2c5d788, {
    onEnter(args) {
      console.log("[Review] GetServerConfig called!");
    },
  });

  hook("UserAuthComposite.UserAuth", 0x2c5df1c, {
    onEnter(args) {
      console.log("[Auth] UserAuth called!");
    },
  });

  hook("UserAuthComposite.DoAuth", 0x2c5dfb8, {
    onEnter(args) {
      console.log("[Auth] DoAuth called!");
    },
  });

  // ---- CHANNEL CREATION ----

  hook("Channel.ctor", 0x35ffa94, {
    onEnter(args) {
      try {
        const target = readStr(args[1]);
        console.log(`[Channel] NEW channel to: "${target}"`);
        writeStr(args[1], `${SERVER_ADDRESS}:${SERVER_PORT}`);
        console.log(
          `[Channel] Redirected to: ${SERVER_ADDRESS}:${SERVER_PORT}`
        );
      } catch (e) {
        console.log("[Channel] ctor hook error:", e);
      }
    },
  });

  // ---- SERVER REDIRECT ----

  hook("ChannelProvider.Setup", 0x35ff61c, {
    onEnter(args) {
      const orig = readStr(args[0]);
      console.log(`[Setup] ${orig} -> ${SERVER_ADDRESS}:${SERVER_PORT}`);
      writeStr(args[0], `${SERVER_ADDRESS}:${SERVER_PORT}`);
    },
  });

  hook("ChannelProvider.get_Channel", 0x35ff440, {
    onEnter(args) {
      console.log("[Channel] get_Channel called");
    },
  });

  hook("ChannelProvider.Initialize", 0x35ff4d0, {
    onEnter(args) {
      console.log("[Channel] Initialize called");
    },
  });

  hook("ServerResolver.SetConfiguration", 0x3602b8c, {
    onEnter(args) {
      try {
        const config = args[1];
        if (!config.isNull()) {
          const serverPtr = config.add(0x10).readPointer();
          const reviewPtr = config.add(0x18).readPointer();
          console.log(
            `[SetConfig] Server=${readStr(serverPtr)} Review=${readStr(
              reviewPtr
            )}`
          );
          writeStr(serverPtr, `${SERVER_ADDRESS}:${SERVER_PORT}`);
          writeStr(reviewPtr, `${SERVER_ADDRESS}:${SERVER_PORT}`);
        }
      } catch (e) {
        console.log("[SetConfig] error:", e);
      }
    },
  });

  // ---- SSL ----

  hook("GetRootCertificateCredentials", 0x35ff8a8, {
    onEnter(args) {
      console.log("[SSL] GetRootCertificateCredentials called");
    },
  });

  // Force insecure gRPC channel: ChannelCredentialsSafeHandle.ToNativeCredentials -> NULL
  // When NULL, Channel.ctor uses CreateInsecure instead of CreateSecure
  hookReplace(
    "ChannelCredentials.ToNativeCredentials",
    0x3603ff0,
    "pointer",
    ["pointer"],
    function (credentials) {
      console.log(
        "[SSL] ToNativeCredentials -> NULL (forcing insecure channel)"
      );
      return ptr(0);
    }
  );

  // ---- OCTO ----

  hook("DarkOctoSetupper.GetE", 0x366674c, {
    onLeave(retval) {
      const orig = readStr(retval);
      console.log(
        `[OctoURL] ${orig} -> http://${SERVER_ADDRESS}:${HTTP_PORT}/`
      );
      writeStr(retval, `http://${SERVER_ADDRESS}:${HTTP_PORT}/`);
    },
  });

  hook("DarkOctoSetupper.StartSetup", 0x3664344, {
    onEnter(args) {
      console.log("[OctoSetup] StartSetup called");
    },
  });

  hook("DarkOctoSetupper.SetupOcto", 0x3666aac, {
    onEnter(args) {
      console.log("[OctoSetup] SetupOcto called");
    },
  });

  hook("DarkOctoSetupper.CreateSetting", 0x3666b9c, {
    onEnter(args) {
      console.log("[OctoSetup] CreateSetting called");
    },
  });

  // OctoManager.Setup — RVA: 0x4C02890
  hook("OctoManager.Setup", 0x4c02890, {
    onEnter(args) {
      console.log("[OctoManager] Setup called");
    },
    onLeave(retval) {
      console.log(`[OctoManager] Setup returned: ${retval}`);
    },
  });

  // OctoManager._Setup — RVA: 0x4C02B44
  hook("OctoManager._Setup", 0x4c02b44, {
    onEnter(args) {
      console.log("[OctoManager] _Setup called");
    },
    onLeave(retval) {
      console.log(`[OctoManager] _Setup returned: ${retval}`);
    },
  });

  // OctoManager.StartDbUpdate — RVA: 0x4C041B8
  // Triggers DecryptAes bypass on first call (metadata guaranteed loaded by now)
  hook("OctoManager.StartDbUpdate", 0x4c041b8, {
    onEnter(args) {
      console.log(
        `[OctoManager] StartDbUpdate called naturally (callback=${args[0]} reset=${args[1]})`
      );
      if (!globalThis._octoAesBypassed) {
        globalThis._octoAesBypassed = true;
        bypassDecryptAes();
      }
    },
  });

  // ---- ENCRYPTION BYPASS ----

  hookReplace(
    "HandleNet.Encrypt",
    0x277580c,
    "pointer",
    ["pointer", "pointer"],
    function (thisArg, payload) {
      console.log("[Crypto] Encrypt bypassed");
      return payload;
    }
  );

  hookReplace(
    "HandleNet.Decrypt",
    0x277590c,
    "pointer",
    ["pointer", "pointer"],
    function (thisArg, receivedMessage) {
      console.log("[Crypto] Decrypt bypassed");
      return receivedMessage;
    }
  );

  // HandleNet.DecryptMasterData — let it run naturally!
  // Server now serves REAL encrypted database.bin.e from archive.org.
  // The game's built-in AES decryption will handle it.
  hook("HandleNet.DecryptMasterData", 0x2775a0c, {
    onEnter(args) {
      console.log("[Crypto] DecryptMasterData called (NATURAL)");
    },
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
            tableCount =
              (dataStart.add(1).readU8() << 8) | dataStart.add(2).readU8();
          } else if (mapByte === 0xdf) {
            tableCount = dataStart.add(1).readU32();
          }
          console.log(
            `[MasterData] Table count: ${tableCount} (map byte: 0x${mapByte.toString(
              16
            )})`
          );
        }
      } catch (e) {
        console.log("[MasterData] Parse error: " + e);
      }
    },
  });

  // MemoryDatabaseBase.ctor — NO HOOK (Interceptor.attach on ctors may corrupt completion chains)
  console.log("[*] MemoryDatabaseBase.ctor: NO HOOK (safety)");

  // ---- GRPC LOGGING ----

  hook("DarkClient.InvokeAsync", 0x38743fc, {
    onEnter(args) {
      try {
        const path = readStr(args[1]);
        console.log(`[gRPC] >>> ${path}`);
      } catch (e) {
        console.log("[gRPC] >>> (call detected)");
      }
    },
  });

  hook("DarkClient.ctor", 0x27a3134, {
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
        console.log(
          `[DarkClient] deadline extended: ${oldSec.toFixed(1)}s -> 300s`
        );
      } catch (e) {
        console.log("[DarkClient] deadline patch error: " + e);
      }
    },
  });

  hook("ErrorHandlingInterceptor.SendAsync", 0x2e117bc, {
    onEnter(args) {
      console.log("[ErrorHandler] SendAsync called");
    },
  });

  hook("ErrorHandlingInterceptor.ErrorHandling", 0x2e118e0, {
    onEnter(args) {
      try {
        const ex = args[1]; // RpcException
        // RpcException.status at offset 0x88 (Status struct)
        // Status contains: StatusCode (int enum) at +0x0, Detail (string) at +0x8
        const statusCode = ex.add(0x88).readS32();
        const detailPtr = ex.add(0x88 + 0x8).readPointer();
        let detail = "<null>";
        if (!detailPtr.isNull()) detail = readStr(detailPtr);
        console.log(
          `[ErrorHandler] RPC Exception: statusCode=${statusCode} detail="${detail}"`
        );
      } catch (e) {
        console.log(`[ErrorHandler] ErrorHandling (read err: ${e})`);
      }
    },
  });

  // HandleNet error methods
  hook("HandleNet.ShowStayError", 0x2776130, {
    onEnter(args) {
      try {
        console.log(
          `[HandleNet] ShowStayError: msg=${readStr(args[1])} code=${readStr(
            args[3]
          )}`
        );
      } catch (e) {
        console.log("[HandleNet] ShowStayError called");
      }
    },
  });
  hook("HandleNet.ShowMoveTitleError", 0x27762b4, {
    onEnter(args) {
      try {
        console.log(
          `[HandleNet] ShowMoveTitleError: msg=${readStr(
            args[1]
          )} code=${readStr(args[3])}`
        );
      } catch (e) {
        console.log("[HandleNet] ShowMoveTitleError called");
      }
    },
  });
  hook("HandleNet.ShowErrorDialog", 0x27771c0, {
    onEnter(args) {
      try {
        console.log(
          `[HandleNet] ShowErrorDialog: msg=${readStr(args[1])} arg=${readStr(
            args[2]
          )} code=${readStr(args[3])}`
        );
      } catch (e) {
        console.log("[HandleNet] ShowErrorDialog called");
      }
    },
  });
  hook("HandleNet.ShowAuthRetryError", 0x27764c0, {
    onEnter(args) {
      console.log("[HandleNet] ShowAuthRetryError called");
    },
  });
  hook("HandleNet.ShowClientFatalError", 0x277655c, {
    onEnter(args) {
      try {
        console.log(`[HandleNet] ShowClientFatalError: ${readStr(args[1])}`);
      } catch (e) {
        console.log("[HandleNet] ShowClientFatalError called");
      }
    },
  });
  hook("HandleNet.ShowMoveTitleMaintenanceError", 0x2776388, {
    onEnter(args) {
      console.log("[HandleNet] ShowMoveTitleMaintenanceError called");
    },
  });

  // CommonDialogManager.Show — RVA: 0x30459AC
  hook("CommonDialogManager.Show", 0x30459ac, {
    onEnter(args) {
      try {
        const msgKey = readStr(args[2]);
        const title = readStr(args[3]);
        const okBtn = readStr(args[4]);
        console.log(
          `[DIALOG] Show: key="${msgKey}" title="${title}" ok="${okBtn}"`
        );
      } catch (e) {
        console.log("[DIALOG] Show called (read error)");
      }
    },
  });

  // CommonDialogManager.ShowWithParams — RVA: 0x3045AC8
  hook("CommonDialogManager.ShowWithParams", 0x3045ac8, {
    onEnter(args) {
      try {
        const msgKey = readStr(args[2]);
        const msgArg = readStr(args[3]);
        const title = readStr(args[4]);
        console.log(
          `[DIALOG] ShowWithParams: key="${msgKey}" arg="${msgArg}" title="${title}"`
        );
      } catch (e) {
        console.log("[DIALOG] ShowWithParams called (read error)");
      }
    },
  });

  // MessageDialogPresenter.SetData — RVA: 0x302696C
  hook("MessageDialogPresenter.SetData", 0x302696c, {
    onEnter(args) {
      try {
        const title = readStr(args[1]);
        const message = readStr(args[2]);
        const okBtn = readStr(args[3]);
        const cancelBtn = readStr(args[4]);
        console.log(
          `[DIALOG-SET] title="${title}" message="${message}" ok="${okBtn}" cancel="${cancelBtn}"`
        );
      } catch (e) {
        console.log("[DIALOG-SET] SetData called (read error):", e);
      }
    },
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
  console.log(
    "[*] SyncMasterDataAndUserData.MoveNext: no hook (completion safety)"
  );

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
  (function () {
    const addr = libil2cpp.add(0x30a9aac);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result=1, token=0)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source=null -> completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log(
      "[*] Patch Title.FetchTermsOfServiceVersion -> instant complete (version=1)"
    );
  })();

  // FSM.UpdateEvent.MoveNext (0x423C018) — NO HOOK.
  // Interceptor.attach on MoveNext corrupts jump tables,
  // breaking OnCompleteTransition dispatch in the last transition.
  console.log("[*] FSM.UpdateEvent.MoveNext: no hook (jump table safety)");

  // FSM.RequestUpdate hook REMOVED — BL patches call this directly,
  // Interceptor trampoline crashes with SIGSEGV (x8/MethodInfo=null for shared generic).
  // Log RequestUpdate via JS NativeFunction wrapper instead:
  const _RequestUpdate = new NativeFunction(libil2cpp.add(0x423d24c), "void", [
    "pointer",
    "int32",
  ]);
  globalThis._RequestUpdate = _RequestUpdate;

  // Title.OnComplete (0x30A9260) — NO HOOK (corrupts function).
  console.log("[*] Title.OnComplete: no hook");

  // Gameplay.CreateTitleAsync — RVA: 0x274A8D4
  hook("Gameplay.CreateTitleAsync", 0x274a8d4, {
    onEnter(args) {
      console.log("[Gameplay] CreateTitleAsync called, self=" + args[0]);
      globalThis._gameplayInstance = args[0];
    },
  });

  // Gameplay.DisposeTitle — RVA: 0x274A77C
  hook("Gameplay.DisposeTitle", 0x274a77c, {
    onEnter(args) {
      console.log("[Gameplay] DisposeTitle called");
    },
  });

  // UniTask.WaitUntil — RVA: 0x... (static method)
  // Gameplay.<CreateTitleAsync>d__415.MoveNext — RVA: 0x289588C
  // NO HOOK — MoveNext hooks corrupt async state machine jump tables.
  console.log("[*] Gameplay.CreateTitleAsync.MoveNext: no hook (safety)");

  hook("Generator.SetupApiSystem", 0x2e96dbc, {
    onEnter(args) {
      console.log("[Generator] SetupApiSystem!");
    },
  });

  hook("Generator.OnEntrypoint", 0x2e966a8, {
    onEnter(args) {
      console.log("[Generator] OnEntrypoint!");
    },
  });

  // Dark.Kernel.StateMachine.SetupStateMachine — RVA: 0x2AA440C
  // Confirmed: creates HandleNet + Gameplay FSMs at startup.
  console.log(
    "[*] StateMachine.SetupStateMachine: no hook (confirmed working)"
  );

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
  hook("Gameplay.StartGameplayStateMachine", 0x274e478, {
    onEnter(args) {
      console.log("[Gameplay] StartGameplayStateMachine called!");
    },
  });

  hook("Gameplay.GetFirstGameplayEvent", 0x274e780, {
    onLeave(retval) {
      const EVENTS = {
        0: "Unknown",
        1: "RunDevelopmentMenu",
        2: "RunApplication",
        3: "LockApplication",
        4: "StartGameplay",
        5: "StartMainStory",
        6: "StartDevActorViewer",
        7: "StartDevUIViewer",
        8: "StartDevMinigame",
      };
      const ev = retval.toInt32();
      console.log(
        `[Gameplay] GetFirstGameplayEvent -> ${ev} (${EVENTS[ev] || "?"})`
      );
    },
  });

  // OnRunApplicationAsync.MoveNext — NO HOOK (MoveNext hooks corrupt jump tables)
  console.log(
    "[*] OnRunApplicationAsync.MoveNext: NO HOOK (completion safety)"
  );

  // OnTitleAsync.MoveNext — NO HOOK (MoveNext hooks corrupt jump tables)
  // Previously this hook was suspected of causing OnTitleAsync to stall.
  console.log("[*] OnTitleAsync.MoveNext: NO HOOK (completion safety)");

  // RequestUpdate: no patch (natural flow)
  console.log("[*] RequestUpdate: no patch (natural flow)");

  // OnMainStoryAsync: DELAYED START — wait for database to be ready.
  // The database builds asynchronously after SyncUserData completes.
  // We wait until ActivePlayerToEntityMainQuestStatus returns non-NULL.
  console.log("[*] OnMainStoryAsync: delayed start (waiting for DB ready)");
  hook("Gameplay.OnMainStoryAsync", 0x274e4d4, {
    onEnter(args) {
      console.log("[TRACE] OnMainStoryAsync ENTER (checking DB readiness...)");
      // Check if database is ready by calling ActivePlayerToEntityMainQuestStatus
      const getStatus = new NativeFunction(
        libil2cpp.add(0x2ab491c),
        "pointer",
        []
      );
      let attempts = 0;
      let status = getStatus();
      while (status.isNull() && attempts < 50) {
        // Sleep 100ms
        const start = Date.now();
        while (Date.now() - start < 100) {}
        status = getStatus();
        attempts++;
      }
      console.log(
        `[TRACE] OnMainStoryAsync: DB ready after ${attempts} attempts, status=${status}`
      );
    },
    onLeave(retval) {
      console.log("[TRACE] OnMainStoryAsync LEAVE");
    },
  });

  // ---- OnMainStoryAsync FLOW TRACING ----
  // WaitCompletionScene — RVA: 0x2748B0C (polls cwsr at Gameplay+0x158)
  hook("Gameplay.WaitCompletionScene", 0x2748b0c, {
    onEnter(args) {
      console.log("[TRACE] WaitCompletionScene ENTER");
    },
    onLeave(retval) {
      console.log("[TRACE] WaitCompletionScene LEAVE");
    },
  });
  hook("Gameplay.EndEventMap", 0x273cf38, {
    onEnter(args) {
      console.log("[Gameplay] EndEventMap called! nextScene=" + args[1]);
    },
    onLeave(retval) {
      console.log("[Gameplay] EndEventMap returned");
    },
  });
  hook("Gameplay.IsNeedsFinishedReturnToTitle", 0x273d208, {
    onLeave(retval) {
      console.log("[Gameplay] IsNeedsFinishedReturnToTitle -> " + retval);
    },
  });
  hook("Gameplay.IsNeedsFinishedTransitionSideStory", 0x273d320, {
    onLeave(retval) {
      console.log("[Gameplay] IsNeedsFinishedTransitionSideStory -> " + retval);
    },
  });
  hook("Gameplay.IsNeedsFinishedTransitionPortal", 0x273d3f4, {
    onLeave(retval) {
      console.log("[Gameplay] IsNeedsFinishedTransitionPortal -> " + retval);
    },
  });
  hook("Gameplay.RunFinishedReturnTitleAsync", 0x273d6c0, {
    onEnter(args) {
      console.log("[Gameplay] RunFinishedReturnTitleAsync called!");
    },
  });
  // BYPASS: DownloadChapterAsync → return completed UniTask<bool>(true)
  // We don't serve chapter assets; IsNeedsChapterAssetDownload forced false wasn't enough — method has other awaits.
  (function () {
    const addr = libil2cpp.add(0x273c3fc);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result = true)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log(
      "[*] PatchCode Gameplay.DownloadChapterAsync → completed UniTask<bool>(true)"
    );
  })();
  hook("Gameplay.IsNeedsChapterAssetDownload", 0x273c598, {
    onLeave(retval) {
      console.log(
        "[Gameplay] IsNeedsChapterAssetDownload -> " +
          retval +
          " (FORCING false)"
      );
      retval.replace(ptr(0));
    },
  });
  hook("Gameplay.NeedsNextPlayingQuest", 0x273d164, {
    onLeave(retval) {
      console.log("[Gameplay] NeedsNextPlayingQuest -> " + retval);
    },
  });
  hook("Gameplay.BeginEventMap", 0x273c86c, {
    onEnter(args) {
      console.log("[Gameplay] BeginEventMap called!");
    },
  });

  hook("Gameplay.RunTitle", 0x274b29c, {
    onEnter(args) {
      globalThis._gameplayPtr = args[0];
      console.log("[Gameplay] RunTitle called! self=" + args[0]);
    },
  });

  // RunTitle.MoveNext — NO HOOK (MoveNext hooks corrupt jump tables)
  console.log("[*] RunTitle.MoveNext: NO HOOK (completion safety)");

  // OnTitleAsync.MoveNext — NO HOOK (MoveNext hooks corrupt jump tables)
  // KEY INSIGHT: This hook was likely the CAUSE of OnTitleAsync stalling.
  console.log("[*] OnTitleAsync.MoveNext: NO HOOK (completion safety)");

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
  console.log(
    "[*] Story.ApplyFirstScene: NO HOOK (MethodInfo* corruption confirmed)"
  );

  // Story.ApplyNewestScene — RVA: 0x27858E8
  // NO HOOK — same reason
  console.log(
    "[*] Story.ApplyNewestScene: NO HOOK (MethodInfo* corruption confirmed)"
  );

  // Debug.LogError(object) — RVA: 0x48D5108
  hook("Debug.LogError", 0x48d5108, {
    onEnter(args) {
      try {
        if (!args[0].isNull()) {
          const klass = args[0].readPointer();
          const toStringMI = klass.add(0x60).readPointer();
          if (!toStringMI.isNull()) {
            const toStringFn = new NativeFunction(toStringMI, "pointer", [
              "pointer",
            ]);
            const str = readStr(toStringFn(args[0]));
            console.log("[Unity-ERROR] " + str);
          }
        }
      } catch (e) {
        console.log("[Unity-ERROR] (unreadable)");
      }
    },
  });

  // Debug.LogException(Exception) — RVA: 0x48CBA38
  hook("Debug.LogException", 0x48cba38, {
    onEnter(args) {
      try {
        if (!args[0].isNull()) {
          console.log("[Unity-EXCEPTION] " + args[0]);
        }
      } catch (e) {
        console.log("[Unity-EXCEPTION] (unreadable)");
      }
    },
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
      console.log(
        `[*] Scanning ${ranges.length} memory ranges for DecryptAes (sync)...`
      );
      for (const range of ranges) {
        let matches;
        try {
          matches = Memory.scanSync(range.base, range.size, pattern);
        } catch (e) {
          continue;
        }
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
          hexBytes.push(
            (
              "0" +
              strAddr
                .shr(b * 8)
                .and(0xff)
                .toUInt32()
                .toString(16)
            ).slice(-2)
          );
        }
        const ptrPattern = hexBytes.join(" ");

        for (const r of Process.enumerateRanges("rw-")) {
          let refs;
          try {
            refs = Memory.scanSync(r.base, r.size, ptrPattern);
          } catch (e) {
            continue;
          }
          for (const ref of refs) {
            const miCandidate = ref.address.sub(0x10);
            try {
              const methodPtr = miCandidate.readPointer();
              const rva = methodPtr.sub(il2cppMod.base);
              if (
                rva.compare(ptr(0)) > 0 &&
                rva.compare(ptr(il2cppMod.size)) < 0
              ) {
                console.log(
                  `[*] DecryptAes: MethodInfo=${miCandidate} RVA=0x${rva.toString(
                    16
                  )}`
                );
                Interceptor.replace(
                  methodPtr,
                  new NativeCallback(
                    function (thisArg, bytes) {
                      console.log("[OctoAES] DecryptAes bypassed");
                      return bytes;
                    },
                    "pointer",
                    ["pointer", "pointer"]
                  )
                );
                console.log("[*] BYPASSED OctoAPI.DecryptAes!");
              }
            } catch (e) {}
          }
        }
      }
    } catch (e) {
      console.log("[!] Octo AES bypass error: " + e);
    }
  }

  // --- DIAGNOSTIC HOOKS: trace OnFirstDownload sub-steps ---

  hook("Title.RequestSynchronousDatabaseAsync", 0x30a8678, {
    onEnter() {
      console.log("[DIAG] RequestSynchronousDatabaseAsync CALLED");
    },
    onLeave(retval) {
      console.log("[DIAG] RequestSynchronousDatabaseAsync returned: " + retval);
    },
  });

  hook("Title.SyncMasterDataAndUserData", 0x30a8760, {
    onEnter() {
      console.log("[DIAG] SyncMasterDataAndUserData CALLED");
    },
    onLeave(retval) {
      console.log("[DIAG] SyncMasterDataAndUserData returned: " + retval);
    },
  });

  hook("Title.SyncMasterData", 0x30a884c, {
    onEnter() {
      console.log("[DIAG] SyncMasterData CALLED");
    },
    onLeave(retval) {
      console.log("[DIAG] SyncMasterData returned: " + retval);
    },
  });

  // Title.SyncUserData — RVA: 0x30A8940
  // DELAYED FORCE: The natural Task→UniTask completion is broken (returns 0).
  // We force completion after a delay to allow database to build in background.
  hook("Title.SyncUserData", 0x30a8940, {
    onEnter() {
      console.log(
        "[DIAG] SyncUserData CALLED (delayed force - waiting 3s for DB build)"
      );
    },
    onLeave(retval) {
      const result = retval.toInt32();
      console.log(`[DIAG] SyncUserData: pre-force result=${result}`);

      // Wait 3 seconds for database to build, then force completion
      setTimeout(() => {
        console.log("[DIAG] SyncUserData: forcing completion after delay");
        // We can't modify retval here (too late), but we can signal externally
        globalThis._syncUserDataCompleted = true;
      }, 3000);

      // Force immediate completion for FSM to advance
      retval.replace(ptr(1)); // x0 = 1 (result = true)
      this.context.x1 = ptr(0); // x1 = null (source = null → completed)
      console.log(
        "[DIAG] SyncUserData: forced completion, DB building in background"
      );
    },
  });

  // Title.SyncPurchase — RVA: 0x30A8A10
  // BYPASS: return completed UniTask<bool>(true) instantly.
  // Unity IAP not available in emulator — skip purchase sync.
  (function () {
    const addr = libil2cpp.add(0x30a8a10);
    Memory.patchCode(addr, 12, (code) => {
      code.writeByteArray([
        0x20,
        0x00,
        0x80,
        0xd2, // mov x0, #1   (result = true)
        0x01,
        0x00,
        0x80,
        0xd2, // mov x1, #0   (source = null → completed)
        0xc0,
        0x03,
        0x5f,
        0xd6, // ret
      ]);
    });
    console.log("[*] BYPASS SyncPurchase -> instant true");
  })();

  hook("Title.GetFirstDownloadSizeAsync", 0x30a93fc, {
    onEnter() {
      console.log("[DIAG] GetFirstDownloadSizeAsync CALLED");
    },
    onLeave(retval) {
      console.log("[DIAG] GetFirstDownloadSizeAsync returned: " + retval);
    },
  });

  // --- DEEPER DIAGNOSTICS: UserDataGet processing ---
  // Previous run confirmed: BuildDB + b__11_1 (success continuation) fire.
  // But HandleSuccess.Invoke and FSM advancement don't happen.
  // Removing all hooks on the async pipeline to eliminate any Interceptor corruption.
  // Only keep safe read-only diagnostics.
  console.log(
    "[*] UserDataGet pipeline: NO HOOKS (confirmed: BuildDB + success continuation fire, but FSM stalls)"
  );

  // Hook il2cpp_raise_exception to catch ALL managed exceptions
  // Note: Module.findExportByName (static) throws "TypeError: not a function" in Frida 17.x.
  // Use instance method Process.getModuleByName().findExportByName() instead.
  try {
    console.log("[*] Looking for il2cpp_raise_exception...");
    const il2cppMod = Process.getModuleByName("libil2cpp.so");
    const raiseEx = il2cppMod.findExportByName("il2cpp_raise_exception");
    const stringCharsAddr = il2cppMod.findExportByName("il2cpp_string_chars");
    console.log(
      "[*] il2cpp_raise_exception=" +
        raiseEx +
        " il2cpp_string_chars=" +
        stringCharsAddr
    );
    let getChars = null;
    try {
      if (stringCharsAddr)
        getChars = new NativeFunction(stringCharsAddr, "pointer", ["pointer"]);
    } catch (nfErr) {
      console.log("[*] NativeFunction(il2cpp_string_chars) failed: " + nfErr);
    }
    if (raiseEx && !raiseEx.isNull()) {
      // Read the first 16 bytes to see what the stub looks like
      console.log(
        "[*] il2cpp_raise_exception bytes: " +
          Array.from(new Uint8Array(raiseEx.readByteArray(16)))
            .map((b) => b.toString(16).padStart(2, "0"))
            .join(" ")
      );

      // Try approach 1: follow any branch stub to find the real function
      let realTarget = raiseEx;
      const firstInsn = raiseEx.readU32();
      // Check for B (unconditional branch) instruction: 000101xx
      if ((firstInsn & 0xfc000000) === 0x14000000) {
        const imm26 = firstInsn & 0x03ffffff;
        const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
        realTarget = raiseEx.add(offset);
        console.log(
          "[*] il2cpp_raise_exception is a B stub -> real target at " +
            realTarget
        );
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
              } catch (e2) {}
              console.log("[EXCEPTION] " + className + ": " + msg);
              console.log(
                "[EXCEPTION] stack: " +
                  Thread.backtrace(this.context, Backtracer.ACCURATE)
                    .map(DebugSymbol.fromAddress)
                    .join("\n")
              );
            } catch (e) {
              console.log("[EXCEPTION] (couldn't read: " + e + ")");
            }
          },
        });
        attached = true;
        console.log(
          "[*] Hook il2cpp_raise_exception OK (attach on real target)"
        );
      } catch (e1) {
        console.log(
          "[*] attach on real target failed: " + e1 + ", trying replace..."
        );
      }

      // Approach 2: Interceptor.replace with NativeCallback
      if (!attached) {
        try {
          const original = new NativeFunction(realTarget, "void", ["pointer"]);
          Interceptor.replace(
            raiseEx,
            new NativeCallback(
              function (exObj) {
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
                    } catch (e2) {}
                    console.log("[EXCEPTION] " + className + ": " + msg);
                  }
                } catch (e) {
                  console.log("[EXCEPTION] (couldn't read: " + e + ")");
                }
                original(exObj);
              },
              "void",
              ["pointer"]
            )
          );
          console.log("[*] Hook il2cpp_raise_exception OK (replace)");
        } catch (e2) {
          console.log("[!] il2cpp_raise_exception replace also failed: " + e2);
        }
      }
    } else {
      console.log("[!] il2cpp_raise_exception not found");
    }
  } catch (e) {
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
  hook("UnityWebRequest.Get", 0x5373bbc, {
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
      } catch (e) {
        console.log("[HTTP] GET hook error:", e);
      }
    },
  });

  // UnityWebRequest..ctor(string url, string method, DownloadHandler dH, UploadHandler uH)
  // RVA: 0x5373A4C
  hook("UnityWebRequest.ctor", 0x5373a4c, {
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
      } catch (e) {}
    },
  });

  // UnityWebRequest.SendWebRequest — RVA: 0x5372248
  // Catch ALL outgoing requests, log URL and downgrade https
  hook("UnityWebRequest.SendWebRequest", 0x5372248, {
    onEnter(args) {
      try {
        // args[0] = this (UnityWebRequest instance)
        const getUrlPtr = libil2cpp.add(0x53729e0);
        const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
        const urlStr = getUrl(args[0]);
        const url = readStr(urlStr);
        console.log(`[HTTP] SendWebRequest: ${url}`);
      } catch (e) {
        console.log("[HTTP] SendWebRequest hook error:", e);
      }
    },
  });

  // UnityWebRequest.Head(string uri) — RVA: 0x5373C50
  hook("UnityWebRequest.Head", 0x5373c50, {
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
      } catch (e) {}
    },
  });

  // UnityWebRequest.Post — RVA: 0x5373CBC
  hook("UnityWebRequest.Post", 0x5373cbc, {
    onEnter(args) {
      try {
        const uri = readStr(args[0]);
        console.log(`[HTTP] POST: ${uri}`);
        if (uri.indexOf("nierreincarnation") !== -1) {
          writeStr(args[0], uri.replace("https://", "http://"));
        }
      } catch (e) {}
    },
  });

  // ---- DOWNLOAD HANDLER TEXT INSPECTION ----
  // DownloadHandler.get_text — RVA: 0x5371194
  hook("DownloadHandler.get_text", 0x5371194, {
    onLeave(retval) {
      try {
        if (!retval.isNull()) {
          const text = readStr(retval);
          console.log(
            `[HTTP] downloadHandler.text = "${text.substring(0, 200)}"`
          );
        } else {
          console.log("[HTTP] downloadHandler.text = NULL");
        }
      } catch (e) {
        console.log("[HTTP] downloadHandler.text error:", e);
      }
    },
  });

  // ---- HTTP ERROR INSPECTION ----
  // UnityWebRequest.get_isNetworkError — RVA: 0x53727F4
  hook("UnityWebRequest.get_isNetworkError", 0x53727f4, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      try {
        const getUrlPtr = libil2cpp.add(0x53729e0);
        const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
        const urlStr = getUrl(this.self);
        const url = readStr(urlStr);
        const isErr = retval.toInt32();
        if (isErr !== 0) console.log(`[HTTP] isNetworkError=TRUE url=${url}`);
        else console.log(`[HTTP] isNetworkError=false url=${url}`);
      } catch (e) {
        if (retval.toInt32() !== 0)
          console.log("[HTTP] isNetworkError = TRUE!");
      }
    },
  });

  // UnityWebRequest.get_isHttpError — RVA: 0x5372834
  hook("UnityWebRequest.get_isHttpError", 0x5372834, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      try {
        const getUrlPtr = libil2cpp.add(0x53729e0);
        const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
        const urlStr = getUrl(this.self);
        const url = readStr(urlStr);
        const isErr = retval.toInt32();
        if (isErr !== 0) console.log(`[HTTP] isHttpError=TRUE url=${url}`);
        else console.log(`[HTTP] isHttpError=false url=${url}`);
      } catch (e) {
        if (retval.toInt32() !== 0) console.log("[HTTP] isHttpError = TRUE!");
      }
    },
  });

  // UnityWebRequest.get_error — RVA: 0x53725FC
  hook("UnityWebRequest.get_error", 0x53725fc, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      try {
        if (!retval.isNull()) {
          const err = readStr(retval);
          if (err.length > 0) {
            const getUrlPtr = libil2cpp.add(0x53729e0);
            const getUrl = new NativeFunction(getUrlPtr, "pointer", [
              "pointer",
            ]);
            const urlStr = getUrl(this.self);
            const url = readStr(urlStr);
            console.log(`[HTTP] error="${err}" url=${url}`);
          }
        }
      } catch (e) {}
    },
  });

  // UnityWebRequest.get_responseCode — RVA: 0x5372874
  hook("UnityWebRequest.get_responseCode", 0x5372874, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      try {
        const code = retval.toInt32();
        const getUrlPtr = libil2cpp.add(0x53729e0);
        const getUrl = new NativeFunction(getUrlPtr, "pointer", ["pointer"]);
        const urlStr = getUrl(this.self);
        const url = readStr(urlStr);
        console.log(`[HTTP] responseCode=${code} url=${url}`);
      } catch (e) {
        console.log(`[HTTP] responseCode=${retval.toInt32()}`);
      }
    },
  });

  // ---- FORCE INTERNET REACHABILITY ----
  hookReplace(
    "Application.get_internetReachability",
    0x48cae78,
    "int",
    [],
    function () {
      return 2;
    }
  );

  // TitleStubDelegator.IsValidTermOfService — RVA: 0x28FFAF8
  // Force true to trigger TOS + age verification dialogs.
  // These dialogs set consent flags that OnTermOfService needs for proper completion.
  hook("TitleStubDelegator.IsValidTermOfService", 0x28ffaf8, {
    onLeave(retval) {
      const orig = retval.toInt32();
      retval.replace(ptr(1));
      console.log(`[Delegator] IsValidTermOfService: ${orig} -> forced 1`);
    },
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
        Interceptor.replace(
          abortAddr,
          new NativeCallback(
            function () {
              // Prevent actual abort — gather info first
              const retAddr = this.context.lr;
              const sp = this.context.sp;
              let stackPtrs = [];
              try {
                for (let i = 0; i < 64; i++) {
                  const val = sp.add(i * 8).readPointer();
                  const off = val.sub(il2cppBase);
                  if (
                    off.compare(ptr(0)) >= 0 &&
                    off.compare(ptr(il2cppSize)) < 0
                  ) {
                    stackPtrs.push("+" + i * 8 + "=" + rvaOf(val));
                  }
                }
              } catch (e) {}
              console.log(
                "[ABORT-TRAP] LR=" +
                  rvaOf(retAddr) +
                  " stack_il2cpp=[" +
                  stackPtrs.join(", ") +
                  "]"
              );

              // Now actually abort
              const _raise = new NativeFunction(
                libc.findExportByName("raise"),
                "int",
                ["int"]
              );
              _raise(6);
            },
            "void",
            []
          )
        );
        console.log("[*] abort() REPLACED (trap + continue)");
      }
    } catch (e) {
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
            } catch (e) {}
          },
        });
        console.log("[*] getaddrinfo hook installed!");
      } else {
        console.log("[!] getaddrinfo not found in libc.so");
      }
    } catch (e) {
      console.log("[!] DNS hook error:", e);
      console.log("[!] Error stack:", e.stack);
    }
  }, 1000);

  // ---- DISASSEMBLE ApplyNewestScene (0x27858E8) ----
  // Non-async method: Story.FirstQuestStates ApplyNewestScene(bool omitSideStory)
  // Goal: find BL targets to understand what conditions cause Failure(2)
  setTimeout(() => {
    const base = libil2cpp;
    const rva = 0x27858e8;
    const addr = base.add(rva);
    const maxInsns = 512;
    console.log(
      `\n[DISASM] ApplyNewestScene at ${addr} (RVA=0x${rva.toString(16)})`
    );
    const blTargets = [];
    for (let i = 0; i < maxInsns; i++) {
      const pc = addr.add(i * 4);
      const word = pc.readU32();
      // RET (0xD65F03C0)
      if (word === 0xd65f03c0) {
        console.log(`[DISASM]   +${(i * 4).toString(16)}: RET`);
        break;
      }
      // BL imm26 (opcode 100101)
      if (word >>> 26 === 0x25) {
        const imm26 = word & 0x03ffffff;
        const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
        const target = pc.add(offset);
        const targetRVA = target.sub(base).toInt32();
        blTargets.push({ offset: i * 4, targetRVA: targetRVA });
        console.log(
          `[DISASM]   +${(i * 4).toString(16)}: BL 0x${targetRVA.toString(16)}`
        );
      }
    }
    console.log(
      `[DISASM] ApplyNewestScene: ${blTargets.length} BL calls found`
    );
    blTargets.forEach((t) => {
      console.log(
        `[DISASM]   BL target RVA=0x${t.targetRVA.toString(
          16
        )} (offset +0x${t.offset.toString(16)})`
      );
    });

    // Also disassemble ApplyFirstScene (0x2785888) — shorter, called first
    const rva2 = 0x2785888;
    const addr2 = base.add(rva2);
    console.log(
      `\n[DISASM] ApplyFirstScene at ${addr2} (RVA=0x${rva2.toString(16)})`
    );
    const blTargets2 = [];
    for (let i = 0; i < maxInsns; i++) {
      const pc = addr2.add(i * 4);
      const word = pc.readU32();
      if (word === 0xd65f03c0) {
        console.log(`[DISASM]   +${(i * 4).toString(16)}: RET`);
        break;
      }
      if (word >>> 26 === 0x25) {
        const imm26 = word & 0x03ffffff;
        const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
        const target = pc.add(offset);
        const targetRVA = target.sub(base).toInt32();
        blTargets2.push({ offset: i * 4, targetRVA: targetRVA });
        console.log(
          `[DISASM]   +${(i * 4).toString(16)}: BL 0x${targetRVA.toString(16)}`
        );
      }
    }
    console.log(
      `[DISASM] ApplyFirstScene: ${blTargets2.length} BL calls found`
    );

    // Disassemble ApplyPortalOrMainScene (0x2786508) — called by ApplyNewestScene
    const rva3 = 0x2786508;
    const addr3 = base.add(rva3);
    console.log(
      `\n[DISASM] ApplyPortalOrMainScene at ${addr3} (RVA=0x${rva3.toString(
        16
      )})`
    );
    const blTargets3 = [];
    for (let i = 0; i < maxInsns; i++) {
      const pc = addr3.add(i * 4);
      const word = pc.readU32();
      if (word === 0xd65f03c0) {
        console.log(`[DISASM]   +${(i * 4).toString(16)}: RET`);
        break;
      }
      if (word >>> 26 === 0x25) {
        const imm26 = word & 0x03ffffff;
        const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
        const target = pc.add(offset);
        const targetRVA = target.sub(base).toInt32();
        blTargets3.push({ offset: i * 4, targetRVA: targetRVA });
        console.log(
          `[DISASM]   +${(i * 4).toString(16)}: BL 0x${targetRVA.toString(16)}`
        );
      }
    }
    console.log(
      `[DISASM] ApplyPortalOrMainScene: ${blTargets3.length} BL calls found`
    );

    // Also: ApplyNewestMainScene (0x27869C4)
    const rva4 = 0x27869c4;
    const addr4 = base.add(rva4);
    console.log(
      `\n[DISASM] ApplyNewestMainScene at ${addr4} (RVA=0x${rva4.toString(16)})`
    );
    const blTargets4 = [];
    for (let i = 0; i < maxInsns; i++) {
      const pc = addr4.add(i * 4);
      const word = pc.readU32();
      if (word === 0xd65f03c0) {
        console.log(`[DISASM]   +${(i * 4).toString(16)}: RET`);
        break;
      }
      if (word >>> 26 === 0x25) {
        const imm26 = word & 0x03ffffff;
        const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
        const target = pc.add(offset);
        const targetRVA = target.sub(base).toInt32();
        blTargets4.push({ offset: i * 4, targetRVA: targetRVA });
        console.log(
          `[DISASM]   +${(i * 4).toString(16)}: BL 0x${targetRVA.toString(16)}`
        );
      }
    }
    console.log(
      `[DISASM] ApplyNewestMainScene: ${blTargets4.length} BL calls found`
    );
    console.log(
      "[DISASM] Done. Look up BL targets in dump.cs to understand the logic."
    );
  }, 3000);
});
