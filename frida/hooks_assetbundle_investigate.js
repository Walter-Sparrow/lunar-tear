// Focused Frida trace for post-download assetbundle failures.
// Goal:
// - confirm the client reaches Octo assetbundle load code
// - confirm whether bytes reach Unity AssetBundle.LoadFromMemoryAsync_Internal
// - identify whether the retry dialog comes from asset load failure rather than HTTP failure
//
// Usage:
//   frida -Uf com.square_enix.android_googleplay.nierspww -l frida/hooks_assetbundle_investigate.js

'use strict';

let libil2cpp;

function awaitLibil2cpp(callback) {
  if (globalThis._assetInvestigateHooksInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._assetInvestigateHooksInstalled = true;
    console.log('[*] libil2cpp.so loaded at:', libil2cpp);
    callback();
  } catch (error) {
    setTimeout(() => awaitLibil2cpp(callback), 100);
  }
}

function hook(name, offset, callbacks) {
  const target = libil2cpp.add(offset);
  Interceptor.attach(target, callbacks);
  console.log(`[*] Hook ${name} @ 0x${offset.toString(16)}`);
}

function readManagedString(addr) {
  if (!addr || addr.isNull()) return '<null>';
  try {
    return addr.add(0x14).readUtf16String();
  } catch (e) {
    return '<err>';
  }
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

function readByteArrayLength(arr) {
  try {
    if (!arr || arr.isNull()) return -1;
    return arr.add(0x18).readS32();
  } catch (e) {
    return -1;
  }
}

function readByteArrayPreview(arr, limit) {
  try {
    const len = readByteArrayLength(arr);
    if (len <= 0) return '<empty>';
    const take = Math.min(len, limit);
    return hexdump(arr.add(0x20), { length: take, header: false, ansi: false })
      .trim()
      .replace(/\s+/g, ' ');
  } catch (e) {
    return '<preview-err>';
  }
}

function pointerSummary(ptr) {
  if (!ptr || ptr.isNull()) return '<null>';
  return `${ptr} klass=${safeKlassName(ptr)}`;
}

function readItemSummary(item) {
  try {
    if (!item || item.isNull()) return '<null>';
    const name = readManagedString(item.add(0x18).readPointer());
    const generation = item.add(0x28).readU64();
    const size = item.add(0x30).readS32();
    const crc = item.add(0x34).readU32();
    const state = item.add(0x48).readS32();
    const uploadVersionId = item.add(0x4c).readS32();
    return `name=${name} generation=${generation} size=${size} crc=${crc} state=${state} uploadVersionId=${uploadVersionId}`;
  } catch (e) {
    return `<item-err ${e}>`;
  }
}

function readNamedErrorSummary(err) {
  try {
    if (!err || err.isNull()) return '<null>';
    const name = readManagedString(err.add(0x10).readPointer());
    const code = readManagedString(err.add(0x18).readPointer());
    const message = readManagedString(err.add(0x20).readPointer());
    return `name=${name} code=${code} message=${message}`;
  } catch (e) {
    return `<named-error-err ${e}>`;
  }
}

function logDivider(label) {
  console.log(`\n==== ${label} ====`);
}

awaitLibil2cpp(() => {
  logDivider('Asset Investigation Hooks');

  // Octo static entrypoint used by the game to request an assetbundle by name.
  hook('OctoManager.LoadAssetBundle(internal)', 0x4c312d8, {
    onEnter(args) {
      this.name = readManagedString(args[0]);
      this.isNonCache = args[3].toInt32();
      console.log(
        `[OctoLoad] LoadAssetBundle name=${this.name} isNonCache=${this.isNonCache}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[OctoLoad] LoadAssetBundle requestId=${retval.toInt32()} name=${this.name}`,
      );
    },
  });

  hook('AssetBundleManager.LoadAssetBundle', 0x3d8f478, {
    onEnter(args) {
      this.item = args[1];
      console.log(
        `[ABMgr] LoadAssetBundle item=${pointerSummary(this.item)} onComplete=${args[2]} onProgress=${args[3]}`,
      );
      console.log(`[ABMgr]   ${readItemSummary(this.item)}`);
    },
    onLeave(retval) {
      console.log(`[ABMgr] LoadAssetBundle returned requestId=${retval.toInt32()}`);
    },
  });

  hook('OctoAssetBundleLoader.GetItemByName', 0x4c28bb4, {
    onEnter(args) {
      this.name = readManagedString(args[1]);
      console.log(`[OctoAB] GetItemByName name=${this.name}`);
    },
    onLeave(retval) {
      console.log(`[OctoAB] GetItemByName -> ${pointerSummary(retval)} name=${this.name}`);
      if (retval && !retval.isNull()) {
        console.log(`[OctoAB]   ${readItemSummary(retval)}`);
      }
    },
  });

  hook('OctoAssetBundleLoader.Download', 0x4c28c1c, {
    onEnter(args) {
      console.log(
        `[OctoAB] Download item=${pointerSummary(args[1])} path=${readManagedString(args[2])} onComplete=${args[3]} onProgress=${args[4]}`,
      );
      console.log(`[OctoAB]   ${readItemSummary(args[1])}`);
    },
  });

  hook('OctoAssetBundleLoader.GetStorageBucket', 0x4c28ca4, {
    onEnter(args) {
      this.item = args[1];
    },
    onLeave(retval) {
      console.log(
        `[OctoAB] GetStorageBucket item=${readItemSummary(this.item)} -> ${readManagedString(retval)}`,
      );
    },
  });

  hook('OctoAssetBundleLoader.GetStorageFileName', 0x4c28cb0, {
    onEnter(args) {
      this.item = args[1];
    },
    onLeave(retval) {
      console.log(
        `[OctoAB] GetStorageFileName item=${readItemSummary(this.item)} -> ${readManagedString(retval)}`,
      );
    },
  });

  hook('OctoAssetBundleLoader.LoadFromCacheOrDownload', 0x4c28cbc, {
    onEnter(args) {
      this.assetBundleName = readManagedString(args[1]);
      console.log(
        `[OctoAB] LoadFromCacheOrDownload name=${this.assetBundleName} onComplete=${args[2]} onProgress=${args[3]}`,
      );
    },
  });

  hook('OctoAssetBundleLoader.DownloadToMemory', 0x4c29314, {
    onEnter(args) {
      this.assetBundleName = readManagedString(args[1]);
      console.log(
        `[OctoAB] DownloadToMemory name=${this.assetBundleName} onComplete=${args[2]} onProgress=${args[3]}`,
      );
    },
  });

  // Error callback for LoadFromCacheOrDownload.
  hook('OctoAssetBundleLoader.<LoadFromCacheOrDownload>b__0', 0x4c296dc, {
    onEnter(args) {
      console.log(
        `[OctoAB] LoadFromCacheOrDownload callback err=${pointerSummary(args[1])}`,
      );
      console.log(`[OctoAB]   ${readNamedErrorSummary(args[1])}`);
    },
  });

  // Result callback for DownloadToMemory. Result<byte[]> layout is unknown here, so log raw pointers/klass.
  hook('OctoAssetBundleLoader.<DownloadToMemory>b__0', 0x4c297c0, {
    onEnter(args) {
      console.log(
        `[OctoAB] DownloadToMemory callback result=${pointerSummary(args[1])}`,
      );
    },
  });

  // Unity native assetbundle constructors.
  hook('AssetBundle.LoadFromFileAsync_Internal', 0x540132c, {
    onEnter(args) {
      this.path = readManagedString(args[0]);
      console.log(
        `[UnityAB] LoadFromFileAsync_Internal path=${this.path} crc=${args[1].toUInt32()} offset=${args[2]}`,
      );
    },
    onLeave(retval) {
      console.log(`[UnityAB] LoadFromFileAsync_Internal -> ${pointerSummary(retval)}`);
    },
  });

  hook('AssetBundle.LoadFromMemoryAsync_Internal', 0x54013cc, {
    onEnter(args) {
      this.binary = args[0];
      const len = readByteArrayLength(this.binary);
      const preview = readByteArrayPreview(this.binary, 16);
      console.log(
        `[UnityAB] LoadFromMemoryAsync_Internal len=${len} crc=${args[1].toUInt32()} preview=${preview}`,
      );
    },
    onLeave(retval) {
      console.log(`[UnityAB] LoadFromMemoryAsync_Internal -> ${pointerSummary(retval)}`);
    },
  });

  hook('AssetBundle.LoadAssetAsync', 0x54017b4, {
    onEnter(args) {
      const assetName = readManagedString(args[1]);
      console.log(
        `[UnityAB] LoadAssetAsync bundle=${pointerSummary(args[0])} asset=${assetName} type=${pointerSummary(args[2])}`,
      );
    },
    onLeave(retval) {
      console.log(`[UnityAB] LoadAssetAsync -> ${pointerSummary(retval)}`);
    },
  });

  // Rollback path used when an assetbundle request fails after being queued.
  hook('AssetBundleManager.RequestRollBack.Call', 0x3d91968, {
    onEnter(args) {
      console.log(
        `[ABMgr] RequestRollBack.Call op=${pointerSummary(args[1])} err=${pointerSummary(args[2])}`,
      );
      console.log(`[ABMgr]   ${readNamedErrorSummary(args[2])}`);
    },
  });

  hook('ErrorExtensions.ToNamedError', 0x3da5cf8, {
    onEnter(args) {
      this.nameArg = readManagedString(args[1]);
      console.log(
        `[OctoErr] ToNamedError error=${pointerSummary(args[0])} nameArg=${this.nameArg}`,
      );
    },
    onLeave(retval) {
      console.log(`[OctoErr] ToNamedError -> ${readNamedErrorSummary(retval)}`);
    },
  });

  hook('NamedError.CreateDatabaseNotFoundError', 0x4c29d18, {
    onEnter(args) {
      this.name = readManagedString(args[0]);
      console.log(`[OctoErr] CreateDatabaseNotFoundError name=${this.name}`);
    },
    onLeave(retval) {
      console.log(`[OctoErr] CreateDatabaseNotFoundError -> ${readNamedErrorSummary(retval)}`);
    },
  });

  // HandleNet-side retry dialog path for asset/download errors.
  hook('HandleNet.ShowRetryErrorDialog', 0x2794fbc, {
    onEnter() {
      console.log('[HandleNet] ShowRetryErrorDialog');
    },
  });

  hook('HandleNet.ShowDownloadErrorDialog', 0x2795058, {
    onEnter(args) {
      console.log(`[HandleNet] ShowDownloadErrorDialog statusCode=${args[1].toInt32()}`);
    },
  });

  hook('HandleNet.ShowAssetErrorRetryDialog', 0x2795104, {
    onEnter(args) {
      console.log(
        `[HandleNet] ShowAssetErrorRetryDialog downloadError=${pointerSummary(args[1])} retryCount=${args[2].toInt32()} retryAction=${args[3]}`,
      );
    },
  });

  // Generic retry dialog entry used elsewhere in gameplay.
  hook('DialogHelper.ShowDialogQuestRetry', 0x2ed8318, {
    onEnter() {
      console.log('[Dialog] ShowDialogQuestRetry');
    },
  });
});
