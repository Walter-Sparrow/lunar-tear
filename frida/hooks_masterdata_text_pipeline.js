'use strict';

// Focused Frida trace for the current 40% loading stall.
//
// Goal:
// - verify master data really completes end-to-end
// - verify Dark.Master gets assigned
// - verify the client starts consuming downloaded localization text bundles
// - keep the older assetbundle investigation script untouched
//
// Usage:
//   frida -Uf com.square_enix.android_googleplay.nierspww -l frida/hooks_masterdata_text_pipeline.js

let libil2cpp;

function awaitLibil2cpp(callback) {
  if (globalThis._masterDataTextHooksInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._masterDataTextHooksInstalled = true;
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
  } catch (error) {
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
  } catch (error) {
    return '<klass-err>';
  }
}

function pointerSummary(ptr) {
  if (!ptr || ptr.isNull()) return '<null>';
  return `${ptr} klass=${safeKlassName(ptr)}`;
}

function readByteArrayLength(arr) {
  try {
    if (!arr || arr.isNull()) return -1;
    return arr.add(0x18).readS32();
  } catch (error) {
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
  } catch (error) {
    return '<preview-err>';
  }
}

function readRangeSummary(rawValue) {
  try {
    // MasterDataDownloader.Range is two int32 values packed into 8 bytes.
    const raw = BigInt(rawValue.toString());
    const start = Number(raw & 0xffffffffn) | 0;
    const length = Number((raw >> 32n) & 0xffffffffn) | 0;
    return `start=${start} length=${length} end=${start + length - 1}`;
  } catch (error) {
    return `raw=${rawValue}`;
  }
}

function readNamedErrorSummary(err) {
  try {
    if (!err || err.isNull()) return '<null>';
    const name = readManagedString(err.add(0x10).readPointer());
    const code = readManagedString(err.add(0x18).readPointer());
    const message = readManagedString(err.add(0x20).readPointer());
    return `name=${name} code=${code} message=${message}`;
  } catch (error) {
    return `<named-error-err ${error}>`;
  }
}

function readBoxedInt32(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return null;
    return obj.add(0x10 + fieldOffset).readS32();
  } catch (error) {
    return null;
  }
}

function readBoxedPointer(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return ptr(0);
    return obj.add(0x10 + fieldOffset).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function logDivider(label) {
  console.log(`\n==== ${label} ====`);
}

awaitLibil2cpp(() => {
  logDivider('Master DB Hooks');

  // Master data RPC / response / downloader flow.
  hook('DataServiceClient.GetLatestMasterDataVersionAsync', 0x3a43788, {
    onEnter(args) {
      console.log(
        `[MasterRPC] GetLatestMasterDataVersionAsync client=${pointerSummary(args[0])} request=${pointerSummary(args[1])}`,
      );
    },
    onLeave(retval) {
      console.log(`[MasterRPC] GetLatestMasterDataVersionAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('MasterDataGetLatestVersionResponse.get_LatestMasterDataVersion', 0x3a46a20, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      console.log(
        `[MasterRPC] LatestMasterDataVersion self=${pointerSummary(this.self)} -> ${readManagedString(retval)}`,
      );
    },
  });

  hook('MasterDataDownloader.DownloadAsync', 0x3371d54, {
    onEnter(args) {
      console.log(`[MasterDL] DownloadAsync cancellationTokenPtr=${args[0]}`);
    },
    onLeave(retval) {
      console.log(`[MasterDL] DownloadAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('MasterDataDownloader.TryGetMasterDataCacheAsync', 0x33721d0, {
    onEnter(args) {
      this.version = readManagedString(args[0]);
      const head = args[2];
      console.log(
        `[MasterDL] TryGetMasterDataCacheAsync version=${this.version} contentLength=${args[1].toInt32()} headLen=${readByteArrayLength(head)} headPreview=${readByteArrayPreview(head, 16)}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[MasterDL] TryGetMasterDataCacheAsync -> ${pointerSummary(retval)} version=${this.version}`,
      );
    },
  });

  hook('MasterDataDownloader.WriteMasterDataCacheAsync', 0x33722f0, {
    onEnter(args) {
      const data = args[1];
      console.log(
        `[MasterDL] WriteMasterDataCacheAsync version=${readManagedString(args[0])} dataLen=${readByteArrayLength(data)}`,
      );
    },
  });

  // Database construction and publication.
  hook('MemoryDatabaseBase.ctor(byte[])', 0x3bdc30c, {
    onEnter(args) {
      this.self = args[0];
      const data = args[1];
      console.log(
        `[MemoryDB] base ctor self=${pointerSummary(args[0])} dataLen=${readByteArrayLength(data)} internString=${args[2].toInt32()} formatterResolver=${pointerSummary(args[3])} maxDegree=${args[4].toInt32()}`,
      );
    },
    onLeave() {
      console.log(`[MemoryDB] base ctor completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('DarkMasterMemoryDatabase.ctor(byte[])', 0x26abb18, {
    onEnter(args) {
      this.self = args[0];
      const data = args[1];
      console.log(
        `[MasterDB] ctor self=${pointerSummary(args[0])} dataLen=${readByteArrayLength(data)} preview=${readByteArrayPreview(data, 16)}`,
      );
    },
    onLeave() {
      console.log(`[MasterDB] ctor completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('DarkMasterMemoryDatabase.Init', 0x26abb24, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[MasterDB] Init self=${pointerSummary(args[0])} header=${pointerSummary(args[1])} databaseBinary=${args[2]} options=${pointerSummary(args[3])} maxDegree=${args[4].toInt32()}`,
      );
    },
    onLeave() {
      console.log(`[MasterDB] Init completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('DarkMasterMemoryDatabase.InitSequential', 0x26abb34, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[MasterDB] InitSequential self=${pointerSummary(args[0])} header=${pointerSummary(args[1])} databaseBinary=${args[2]} options=${pointerSummary(args[3])} maxDegree=${args[4].toInt32()}`,
      );
    },
    onLeave() {
      console.log(`[MasterDB] InitSequential completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('DarkMasterMemoryDatabase.InitParallel', 0x26c5cfc, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[MasterDB] InitParallel self=${pointerSummary(args[0])} header=${pointerSummary(args[1])} databaseBinary=${args[2]} options=${pointerSummary(args[3])} maxDegree=${args[4].toInt32()}`,
      );
    },
    onLeave() {
      console.log(`[MasterDB] InitParallel completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('MasterDataDownloader.<DownloadAsync>b__2', 0x3372ba4, {
    onEnter(args) {
      console.log(`[MasterDL] <DownloadAsync>b__2 self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[MasterDL] <DownloadAsync>b__2 -> ${pointerSummary(retval)}`);
    },
  });

  hook('DatabaseDefine.set_Master', 0x2f45ccc, {
    onEnter(args) {
      console.log(`[MasterDB] set_Master value=${pointerSummary(args[0])}`);
    },
  });

  // User data hand-off after master data succeeds.
  hook('UserDataGet.RequestAsync', 0x361ad5c, {
    onEnter(args) {
      console.log(`[UserDB] RequestAsync self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[UserDB] RequestAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('UserDataGet.<RequestAsync>b__0', 0x361b5b0, {
    onEnter(args) {
      console.log(`[UserDB] <RequestAsync>b__0 self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[UserDB] <RequestAsync>b__0 -> ${pointerSummary(retval)}`);
    },
  });

  hook('UserDataGet.<RequestAsync>b__11_1', 0x361ae60, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_1 self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('UserDataGet.<RequestAsync>b__11_3', 0x361b318, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_3 self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('GetUserDataNameV2Api.RequestAsyncMethod', 0x3a461bc, {
    onEnter() {
      console.log('[UserAPI] GetUserDataNameV2Api.RequestAsyncMethod');
    },
    onLeave(retval) {
      console.log(`[UserAPI] GetUserDataNameV2Api.RequestAsyncMethod -> ${pointerSummary(retval)}`);
    },
  });

  hook('GetUserDataApi.RequestAsyncMethod(list)', 0x3a455b8, {
    onEnter(args) {
      console.log(`[UserAPI] GetUserDataApi.RequestAsyncMethod(list) tableNames=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[UserAPI] GetUserDataApi.RequestAsyncMethod(list) -> ${pointerSummary(retval)}`);
    },
  });

  hook('DataServiceClient.GetUserDataAsync', 0x3a43a98, {
    onEnter(args) {
      console.log(
        `[UserRPC] GetUserDataAsync client=${pointerSummary(args[0])} request=${pointerSummary(args[1])}`,
      );
    },
    onLeave(retval) {
      console.log(`[UserRPC] GetUserDataAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('UserDataGetResponse.get_UserDataJson', 0x3a494f4, {
    onEnter(args) {
      this.self = args[0];
    },
    onLeave(retval) {
      console.log(
        `[UserRPC] UserDataGetResponse.get_UserDataJson self=${pointerSummary(this.self)} -> ${pointerSummary(retval)}`,
      );
    },
  });

  hook('GetUserDataApi.<RequestAsyncMethod>d__1.MoveNext', 0x3a45adc, {
    onEnter(args) {
      this.self = args[0];
      const state = readBoxedInt32(args[0], 0x0);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__1.MoveNext self=${pointerSummary(args[0])} state=${state}`,
      );
    },
    onLeave() {
      const state = readBoxedInt32(this.self, 0x0);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__1.MoveNext completed self=${pointerSummary(this.self)} state=${state}`,
      );
    },
  });

  hook('GetUserDataApi.<RequestAsyncMethod>d__0.MoveNext', 0x3a456b8, {
    onEnter(args) {
      this.self = args[0];
      const state = readBoxedInt32(args[0], 0x0);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__0.MoveNext self=${pointerSummary(args[0])} state=${state}`,
      );
    },
    onLeave() {
      const state = readBoxedInt32(this.self, 0x0);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__0.MoveNext completed self=${pointerSummary(this.self)} state=${state}`,
      );
    },
  });

  hook('UserDataGet.<RequestAsync>d__11.MoveNext', 0x361b624, {
    onEnter(args) {
      this.self = args[0];
      const state = readBoxedInt32(args[0], 0x0);
      const displayClass = readBoxedPointer(args[0], 0x28);
      const databaseBuilder = readBoxedPointer(args[0], 0x38);
      console.log(
        `[UserDB] <RequestAsync>d__11.MoveNext self=${pointerSummary(args[0])} state=${state} displayClass=${pointerSummary(displayClass)} databaseBuilder=${pointerSummary(databaseBuilder)}`,
      );
    },
    onLeave() {
      const state = readBoxedInt32(this.self, 0x0);
      const displayClass = readBoxedPointer(this.self, 0x28);
      const databaseBuilder = readBoxedPointer(this.self, 0x38);
      console.log(
        `[UserDB] <RequestAsync>d__11.MoveNext completed self=${pointerSummary(this.self)} state=${state} displayClass=${pointerSummary(displayClass)} databaseBuilder=${pointerSummary(databaseBuilder)}`,
      );
    },
  });

  hook('DatabaseBuilderBase.Build', 0x3bdbd90, {
    onEnter(args) {
      this.self = args[0];
      console.log(`[UserDB] DatabaseBuilderBase.Build self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(
        `[UserDB] DatabaseBuilderBase.Build -> len=${readByteArrayLength(retval)} self=${pointerSummary(this.self)}`,
      );
    },
  });

  hook('DarkUserMemoryDatabase.ctor(byte[])', 0x29c4764, {
    onEnter(args) {
      this.self = args[0];
      const data = args[1];
      console.log(
        `[UserDB] ctor self=${pointerSummary(args[0])} dataLen=${readByteArrayLength(data)} internString=${args[2].toInt32()} formatterResolver=${pointerSummary(args[3])} maxDegree=${args[4].toInt32()}`,
      );
    },
    onLeave() {
      console.log(`[UserDB] ctor completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('DatabaseDefine.set_User', 0x2f45c28, {
    onEnter(args) {
      console.log(`[UserDB] set_User value=${pointerSummary(args[0])}`);
    },
  });

  // Error path that may explain why the user still sees the retry dialog.
  hook('NamedError.CreateDatabaseNotFoundError', 0x4c29d18, {
    onEnter(args) {
      this.name = readManagedString(args[0]);
      console.log(`[Err] CreateDatabaseNotFoundError name=${this.name}`);
    },
    onLeave(retval) {
      console.log(`[Err] CreateDatabaseNotFoundError -> ${readNamedErrorSummary(retval)}`);
    },
  });

  hook('HandleNet.ShowAssetErrorRetryDialog', 0x2795104, {
    onEnter(args) {
      console.log(
        `[HandleNet] ShowAssetErrorRetryDialog downloadError=${pointerSummary(args[1])} retryCount=${args[2].toInt32()} retryAction=${args[3]}`,
      );
    },
  });
});
