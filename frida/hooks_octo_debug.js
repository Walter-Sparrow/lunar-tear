// Minimal Frida script for Octo / title boot + user data debugging.
// - NO URL rewrites
// - NO SSL changes
// - ONLY logging to understand where the title flow / Octo update hangs
//   and how GetUserDataNameV2 / GetUserData behave.
//
// Usage (example):
//   frida -Uf com.square_enix.android_googleplay.nierspww -l frida/hooks_octo_debug.js

'use strict';

const SERVER_ADDRESS = '10.0.2.2'; // used only for logging, not rewriting
const SERVER_PORT = 7777;
const HTTP_PORT = 8080;

let libil2cpp;

function awaitLibil2cpp(callback) {
  if (globalThis._hooksInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._hooksInstalled = true;
    console.log('[*] libil2cpp.so loaded at:', libil2cpp);
    callback();
  } catch (error) {
    setTimeout(() => awaitLibil2cpp(callback), 100);
  }
}

function readStr(addr) {
  if (!addr || addr.isNull()) return '<null>';
  try {
    return addr.add(0x14).readUtf16String();
  } catch (e) {
    return '<err>';
  }
}

function hook(name, offset, callbacks) {
  const ptr = libil2cpp.add(offset);
  Interceptor.attach(ptr, callbacks);
  console.log(`[*] Hook ${name} @ 0x${offset.toString(16)}`);
}

awaitLibil2cpp(() => {
  // ---- TITLE FSM TRACE (no behavior changes) ----

  hook('Title.OnFirstStep', 0x30bde5c, {
    onEnter() {
      console.log('[Title] >>> OnFirstStep');
    },
  });

  hook('Title.OnPreTitle', 0x30bdff8, {
    onEnter() {
      console.log('[Title] >>> OnPreTitle');
    },
  });

  hook('Title.OnTitleScreen', 0x30be5e8, {
    onEnter() {
      console.log('[Title] >>> OnTitleScreen');
    },
  });

  hook('Title.OnApplicationVersion', 0x30bda0c, {
    onEnter() {
      console.log('[Title] >>> OnApplicationVersion');
    },
  });

  hook('Title.OnBanAccount', 0x30bdaa8, {
    onEnter() {
      console.log('[Title] >>> OnBanAccount');
    },
  });

  hook('Title.OnTermOfService', 0x30be2e4, {
    onEnter(args) {
      console.log('[Title] >>> OnTermOfService');
      globalThis._titleInstance = args[0];
      console.log(`[Title] _titleInstance captured: ${args[0]}`);
    },
  });

  hook('Title.OnFirstDownload', 0x30bdc34, {
    onEnter() {
      console.log('[Title] >>> OnFirstDownload');
    },
  });

  hook('Title.InitializeAssetBundles', 0x30bddc0, {
    onEnter() {
      console.log('[Title] >>> InitializeAssetBundles (Octo start)');
    },
  });

  // ---- OCTO / DATA MANAGER TRACE (no behavior changes) ----

  hook('DarkOctoSetupper.GetE', 0x3638fc0, {
    onLeave(retval) {
      const orig = readStr(retval);
      console.log(`[OctoURL] base = ${orig}`);
    },
  });

  hook('DarkOctoSetupper.StartSetup', 0x3636bb8, {
    onEnter() {
      console.log('[OctoSetup] StartSetup called');
    },
  });

  hook('DarkOctoSetupper.SetupOcto', 0x3639320, {
    onEnter() {
      console.log('[OctoSetup] SetupOcto called');
    },
  });

  hook('DarkOctoSetupper.CreateSetting', 0x3639410, {
    onEnter() {
      console.log('[OctoSetup] CreateSetting called');
    },
  });

  hook('OctoManager.Setup', 0x4c2f8bc, {
    onEnter() {
      console.log('[OctoManager] Setup called');
    },
    onLeave(retval) {
      console.log(`[OctoManager] Setup returned: ${retval}`);
    },
  });

  hook('OctoManager._Setup', 0x4c2fb70, {
    onEnter() {
      console.log('[OctoManager] _Setup called');
    },
    onLeave(retval) {
      console.log(`[OctoManager] _Setup returned: ${retval}`);
    },
  });

  hook('OctoManager.StartDbUpdate', 0x4c311e4, {
    onEnter(args) {
      console.log(
        `[OctoManager] StartDbUpdate called (callback=${args[0]} reset=${args[1]})`,
      );
    },
  });

  hook('OctoManager.Internal.GetList', 0x4c26e44, {
    onEnter(args) {
      const revision = args[1].toInt32();
      console.log(`[OctoList] GetList started revision=${revision}`);
    },
  });

  hook('OctoManager.Internal.GetListAes', 0x4c27038, {
    onEnter(args) {
      console.log(
        `[OctoList] GetListAes called revision=${args[1].toInt32()}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[OctoList] GetListAes returned useAes=${retval.toInt32()}`,
      );
    },
  });

  hook('OctoManager.<GetListAes>b__0', 0x4c282a8, {
    onEnter(args) {
      const bytesPtr = args[1];
      const errPtr = args[2];
      const len = bytesPtr.isNull() ? -1 : bytesPtr.add(0x18).readInt();
      console.log(
        `[OctoList] GetListAes callback: bytesLen=${len} hasError=${!errPtr.isNull()}`,
      );
    },
  });

  hook('OctoManager.<StartDbUpdate>b__0', 0x4c316ac, {
    onEnter(args) {
      const dataPtr = args[1];
      const errPtr = args[2];
      const dataLen = dataPtr.isNull() ? -1 : dataPtr.add(0x18).readInt();
      const hasError = !errPtr.isNull();
      console.log(
        `[OctoList] StartDbUpdate callback: dataLen=${dataLen} hasError=${hasError}`,
      );
      if (hasError) {
        try {
          const errMsg = errPtr.add(0x10).readPointer();
          const msgStr = errMsg.isNull() ? '<no msg>' : readStr(errMsg);
          console.log(`[OctoList] Error object: ${msgStr}`);
        } catch (e) {
          console.log('[OctoList] Error parse:', e);
        }
      }
    },
  });

  hook('DataManager.SetUrls', 0x3da0170, {
    onEnter(args) {
      try {
        const db = args[0];
        if (db.isNull()) {
          console.log('[DataManager.SetUrls] db NULL');
          return;
        }
        const revision = db.add(0x10).readInt();
        const assetBundleListPtr = db.add(0x18).readPointer();
        const tagnamePtr = db.add(0x20).readPointer();
        const resourceListPtr = db.add(0x28).readPointer();
        const urlFormatPtr = db.add(0x30).readPointer();

        const safeListCount = (ptr) => {
          try {
            if (ptr.isNull()) return 0;
            return ptr.add(0x10).readInt();
          } catch (e) {
            return -1;
          }
        };

        const urlFormat = urlFormatPtr.isNull()
          ? '<null>'
          : readStr(urlFormatPtr);
        const assetBundleCount = safeListCount(assetBundleListPtr);
        const tagnameCount = safeListCount(tagnamePtr);
        const resourceCount = safeListCount(resourceListPtr);

        console.log(
          `[DataManager.SetUrls] revision=${revision}, urlFormat=${urlFormat}`,
        );
        console.log(
          `[DataManager.SetUrls] assetBundleList=${assetBundleCount}, tagname=${tagnameCount}, resourceList=${resourceCount}`,
        );
      } catch (e) {
        console.log('[DataManager.SetUrls] error:', e);
      }
    },
  });

  hook('DataManager.ApplyToDatabase', 0x3d9f5ec, {
    onEnter(args) {
      this.instance = args[0];
    },
    onLeave() {
      try {
        const inst = this.instance;
        if (!inst || inst.isNull()) {
          console.log('[DataManager.ApplyToDatabase] instance NULL');
          return;
        }
        const revision = inst.add(0x3c).readInt();
        const urlFormatPtr = inst.add(0x40).readPointer();
        const urlFormat = urlFormatPtr.isNull()
          ? '<null>'
          : readStr(urlFormatPtr);
        console.log(
          `[DataManager.ApplyToDatabase] Applied, revision=${revision}, urlFormat=${urlFormat}`,
        );
      } catch (e) {
        console.log('[DataManager.ApplyToDatabase] error:', e);
      }
    },
  });

  // ---- USER DATA / gRPC DIAGNOSTICS ----

  function nowMs() {
    return Date.now();
  }

  function safeKlassName(obj) {
    try {
      if (!obj || obj.isNull()) return '<null>';
      const klass = obj.readPointer();
      if (klass.isNull()) return '<no-klass>';
      const namePtr = klass.add(0x10).readPointer();
      return namePtr.isNull() ? '<no-name>' : namePtr.readCString();
    } catch (e) {
      return '<klass-err>';
    }
  }

  globalThis._userDataDiagUntil = 0;
  function armUserDataWindow(reason) {
    globalThis._userDataDiagUntil = nowMs() + 45000;
    console.log(`[UserDataDiag] window armed (${reason})`);
  }
  function inUserDataWindow() {
    return nowMs() <= globalThis._userDataDiagUntil;
  }

  hook('GetUserDataNameV2Api.RequestAsyncMethod', 0x3a461bc, {
    onEnter() {
      console.log(
        '[UserDataDiag] GetUserDataNameV2Api.RequestAsyncMethod called',
      );
      armUserDataWindow('GetUserDataNameV2');
    },
  });

  hook(
    'GetUserDataApi.RequestAsyncMethod(IReadOnlyList<string>)',
    0x3a455b8,
    {
      onEnter(args) {
        try {
          const listObj = args[0];
          if (!listObj || listObj.isNull()) {
            console.log(
              '[UserDataDiag] GetUserDataApi(list) called with null list',
            );
          } else {
            const size = listObj.add(0x18).readS32();
            console.log(
              `[UserDataDiag] GetUserDataApi.RequestAsyncMethod(list) size=${size} klass=${safeKlassName(
                listObj,
              )}`,
            );
          }
        } catch (e) {
          console.log('[UserDataDiag] GetUserDataApi(list) parse err:', e);
        }
        armUserDataWindow('GetUserDataApi(list)');
      },
    },
  );

  hook('DataServiceClient.GetUserDataAsync(CallOptions)', 0x3a43b30, {
    onEnter(args) {
      console.log(
        `[UserDataDiag] DataServiceClient.GetUserDataAsync called req=${args[0]} reqKlass=${safeKlassName(
          args[0],
        )}`,
      );
      armUserDataWindow('gRPC GetUserDataAsync');
    },
  });

  hook('UserDataGetResponse.get_UserDataJson', 0x3a494f4, {
    onEnter(args) {
      if (!inUserDataWindow()) return;
      console.log(
        `[UserDataDiag] UserDataGetResponse.get_UserDataJson this=${args[0]} klass=${safeKlassName(
          args[0],
        )}`,
      );
    },
  });

  hook('JobResponseHandler.HandleResponseAsync(Api)', 0x35c3470, {
    onEnter(args) {
      if (!inUserDataWindow()) return;
      console.log(
        `[UserDataDiag] JobResponseHandler.HandleResponseAsync api=${args[1]} apiKlass=${safeKlassName(
          args[1],
        )}`,
      );
    },
  });

  hook('JobResponseHandler.HandleBaseResponseAsync(Api)', 0x35c3a60, {
    onEnter(args) {
      if (!inUserDataWindow()) return;
      console.log(
        `[UserDataDiag] JobResponseHandler.HandleBaseResponseAsync api=${args[1]} apiKlass=${safeKlassName(
          args[1],
        )}`,
      );
    },
  });

  hook('JobResponseHandler.HandleSuccessResponseAsync(Api)', 0x35c3a78, {
    onEnter(args) {
      if (!inUserDataWindow()) return;
      console.log(
        `[UserDataDiag] JobResponseHandler.HandleSuccessResponseAsync api=${args[1]} apiKlass=${safeKlassName(
          args[1],
        )}`,
      );
    },
  });

  hook('JobResponseHandler.HandleErrorResponseAsync(Api)', 0x35c3aa8, {
    onEnter(args) {
      if (!inUserDataWindow()) return;
      console.log(
        `[UserDataDiag] JobResponseHandler.HandleErrorResponseAsync api=${args[1]} apiKlass=${safeKlassName(
          args[1],
        )}`,
      );
    },
  });

  // AsyncUniTaskMethodBuilder.SetException — ловим UniTask-исключения (без бектрейса).
  hook('AsyncUniTaskMethodBuilder.SetException', 0x40fcbe4, {
    onEnter(args) {
      try {
        const exc = args[1];
        const typeName = safeKlassName(exc);
        let msg = '';
        if (exc && !exc.isNull()) {
          try {
            const msgObj = exc.add(0x18).readPointer();
            if (!msgObj.isNull()) {
              const len = msgObj.add(0x10).readS32();
              msg = msgObj.add(0x14).readUtf16String(len);
            }
          } catch (e2) {}
        }
        console.log(`[ASYNC-EXC] ${typeName}: ${msg}`);
      } catch (e) {
        console.log('[ASYNC-EXC] SetException parse failed:', e);
      }
    },
  });
});

