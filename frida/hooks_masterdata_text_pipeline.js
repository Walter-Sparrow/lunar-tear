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
let lastUserDataPostCallback = null;
let lastUserDataPostMethod = null;
let userDataRequestSeq = 0;
let activeUserDataRequestId = 0;
let userDataCallbackCtorSeq = 0;
let userDataCallbackMethodsSeen = [];

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

function readRawInt32(rawPtr, fieldOffset) {
  try {
    if (!rawPtr || rawPtr.isNull()) return null;
    return rawPtr.add(fieldOffset).readS32();
  } catch (error) {
    return null;
  }
}

function readRawPointer(rawPtr, fieldOffset) {
  try {
    if (!rawPtr || rawPtr.isNull()) return ptr(0);
    return rawPtr.add(fieldOffset).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function readObjectBool(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return null;
    return obj.add(0x10 + fieldOffset).readU8() !== 0;
  } catch (error) {
    return null;
  }
}

function readObjectPointer(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return ptr(0);
    return obj.add(0x10 + fieldOffset).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function readManagedArrayLength(arr) {
  try {
    if (!arr || arr.isNull()) return -1;
    return arr.add(0x18).readS32();
  } catch (error) {
    return -1;
  }
}

function readManagedArrayElementPointer(arr, index) {
  try {
    if (!arr || arr.isNull()) return ptr(0);
    if (index < 0) return ptr(0);
    return arr.add(0x20 + index * Process.pointerSize).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function readListSize(list) {
  try {
    if (!list || list.isNull()) return -1;
    return list.add(0x18).readS32();
  } catch (error) {
    return -1;
  }
}

function readListItemsArray(list) {
  try {
    if (!list || list.isNull()) return ptr(0);
    return list.add(0x10).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function readListElementPointer(list, index) {
  const items = readListItemsArray(list);
  return readManagedArrayElementPointer(items, index);
}

function readStringListSummary(list, limit = 8) {
  try {
    if (!list || list.isNull()) return '<null>';
    const size = readListSize(list);
    const out = [];
    for (let i = 0; i < Math.min(size, limit); i += 1) {
      out.push(readManagedString(readListElementPointer(list, i)));
    }
    const suffix = size > limit ? ', ...' : '';
    return `size=${size} [${out.join(', ')}${suffix}]`;
  } catch (error) {
    return '<string-list-err>';
  }
}

function readStringListListSummary(listOfLists, outerLimit = 4, innerLimit = 8) {
  try {
    if (!listOfLists || listOfLists.isNull()) return '<null>';
    const size = readListSize(listOfLists);
    const out = [];
    for (let i = 0; i < Math.min(size, outerLimit); i += 1) {
      out.push(`#${i}:${readStringListSummary(readListElementPointer(listOfLists, i), innerLimit)}`);
    }
    const suffix = size > outerLimit ? ', ...' : '';
    return `size=${size} {${out.join(' | ')}${suffix}}`;
  } catch (error) {
    return '<string-list-list-err>';
  }
}

function readTuple2ArraySummary(tupleArray, limit = 4) {
  try {
    if (!tupleArray || tupleArray.isNull()) return '<null>';
    const len = readManagedArrayLength(tupleArray);
    const out = [];
    for (let i = 0; i < Math.min(len, limit); i += 1) {
      const tupleBase = tupleArray.add(0x20 + i * (Process.pointerSize * 2));
      const tableName = readManagedString(tupleBase.readPointer());
      const records = tupleBase.add(Process.pointerSize).readPointer();
      out.push(`${tableName}:records=${readListSize(records)}`);
    }
    const suffix = len > limit ? ', ...' : '';
    return `len=${len} [${out.join(', ')}${suffix}]`;
  } catch (error) {
    return '<tuple-array-err>';
  }
}

function readTuple2Array2DSummary(result, outerLimit = 4, innerLimit = 4) {
  try {
    if (!result || result.isNull()) return '<null>';
    const outerLen = readManagedArrayLength(result);
    const out = [];
    for (let i = 0; i < Math.min(outerLen, outerLimit); i += 1) {
      const inner = readManagedArrayElementPointer(result, i);
      out.push(`#${i}:${readTuple2ArraySummary(inner, innerLimit)}`);
    }
    const suffix = outerLen > outerLimit ? ', ...' : '';
    return `outerLen=${outerLen} {${out.join(' | ')}${suffix}}`;
  } catch (error) {
    return '<tuple-array-2d-err>';
  }
}

function readTuple2ArrayFocusedTableSummary(tupleArray, tableNames) {
  try {
    if (!tupleArray || tupleArray.isNull()) return '<null>';
    const len = readManagedArrayLength(tupleArray);
    const wanted = new Set(tableNames);
    const found = new Map();
    for (let i = 0; i < len; i += 1) {
      const tupleBase = tupleArray.add(0x20 + i * (Process.pointerSize * 2));
      const tableName = readManagedString(tupleBase.readPointer());
      if (!wanted.has(tableName)) continue;
      const records = tupleBase.add(Process.pointerSize).readPointer();
      found.set(tableName, readListSize(records));
    }
    return tableNames
      .map((name) => `${name}=${found.has(name) ? found.get(name) : '<missing>'}`)
      .join(', ');
  } catch (error) {
    return '<focused-tuple-summary-err>';
  }
}

function moduleOffsetHex(addr) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return '<null>';
    return addr.sub(libil2cpp).toString();
  } catch (error) {
    return '<err>';
  }
}

function isLikelyUserDataGetFlowCaller(addr) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return false;
    const offset = addr.sub(libil2cpp).toUInt32();
    return offset >= 0x361ad5c && offset <= 0x361c000;
  } catch (error) {
    return false;
  }
}

function isLikelyUserDataPipelineCaller(addr) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return false;
    const offset = addr.sub(libil2cpp).toUInt32();
    return (
      (offset >= 0x361ad5c && offset <= 0x361c000) ||
      (offset >= 0x3a455b8 && offset <= 0x3a46200)
    );
  } catch (error) {
    return false;
  }
}

function readClientErrorSummary(err) {
  try {
    if (!err || err.isNull()) return '<null>';
    const screenTransitionType = err.add(0x10).readS32();
    return `screenTransitionType=${screenTransitionType} ptr=${err}`;
  } catch (error) {
    return `<client-error-err ${error}>`;
  }
}

function readPointerSlotSummary(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return `+0x${fieldOffset.toString(16)}=<null>`;
    const value = obj.add(0x10 + fieldOffset).readPointer();
    return `+0x${fieldOffset.toString(16)}=${pointerSummary(value)}`;
  } catch (error) {
    return `+0x${fieldOffset.toString(16)}=<err>`;
  }
}

function readDelegateLayoutSummary(obj) {
  return [
    readPointerSlotSummary(obj, 0x0),
    readPointerSlotSummary(obj, 0x8),
    readPointerSlotSummary(obj, 0x10),
    readPointerSlotSummary(obj, 0x18),
    readPointerSlotSummary(obj, 0x20),
    readPointerSlotSummary(obj, 0x28),
  ].join(' ');
}

function readDelegateTarget(obj) {
  try {
    if (!obj || obj.isNull()) return ptr(0);
    return obj.add(0x10 + 0x10).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function pointerLocationSummary(addr) {
  try {
    if (!addr || addr.isNull()) return '<null>';
    const mod = Process.findModuleByAddress(addr);
    if (!mod) return `${addr} <no-module>`;
    if (mod.name === 'libil2cpp.so') {
      return `${addr} ${mod.name}+${moduleOffsetHex(addr)}`;
    }
    return `${addr} ${mod.name}`;
  } catch (error) {
    return `${addr} <loc-err>`;
  }
}

function dumpInstructionWindow(label, centerOffset, beforeCount, afterCount) {
  console.log(`[UserDB] ${label} disassembly around 0x${centerOffset.toString(16)}`);
  for (let i = -beforeCount; i <= afterCount; i += 1) {
    const offset = centerOffset + i * 4;
    try {
      const addr = libil2cpp.add(offset);
      const insn = Instruction.parse(addr);
      const marker = i === 0 ? '>>' : '  ';
      console.log(`[UserDB] ${marker} 0x${offset.toString(16)}: ${insn.mnemonic} ${insn.opStr}`);
    } catch (error) {
      console.log(`[UserDB] !! 0x${offset.toString(16)}: <decode-error ${error}>`);
    }
  }
}

function logDivider(label) {
  console.log(`\n==== ${label} ====`);
}

awaitLibil2cpp(() => {
  logDivider('Master DB Hooks');
  dumpInstructionWindow('UserData post callsite', 0x361bd3c, 10, 6);

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
      userDataRequestSeq += 1;
      activeUserDataRequestId = userDataRequestSeq;
      lastUserDataPostCallback = null;
      lastUserDataPostMethod = null;
      userDataCallbackCtorSeq = 0;
      userDataCallbackMethodsSeen = [];
      console.log(`[UserDB] RequestAsync req=${activeUserDataRequestId} self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[UserDB] RequestAsync req=${activeUserDataRequestId} -> ${pointerSummary(retval)}`);
    },
  });

  hook('Task.WhenAll<TResult[]>(UserDataGet caller)', 0x38af1b4, {
    onEnter(args) {
      if (!isLikelyUserDataGetFlowCaller(this.returnAddress)) return;
      this.shouldLog = true;
      const tasks = args[0];
      const count = readManagedArrayLength(tasks);
      console.log(
        `[UserDB] Task.WhenAll<TResult[]> caller=${moduleOffsetHex(this.returnAddress)} tasks=${pointerSummary(tasks)} count=${count} first0=${pointerSummary(readManagedArrayElementPointer(tasks, 0))} first1=${pointerSummary(readManagedArrayElementPointer(tasks, 1))}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[UserDB] Task.WhenAll<TResult[]> -> ${pointerSummary(retval)}`);
    },
  });

  hook('TaskAwaiter<TResult>.GetResult(UserData pipeline)', 0x4743464, {
    onEnter(args) {
      if (!isLikelyUserDataPipelineCaller(this.returnAddress)) return;
      this.shouldLog = true;
      this.caller = moduleOffsetHex(this.returnAddress);
      const task = readRawPointer(args[0], 0x0);
      console.log(
        `[UserDB] TaskAwaiter<TResult>.GetResult caller=${this.caller} awaiter=${args[0]} task=${pointerSummary(task)}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[UserDB] TaskAwaiter<TResult>.GetResult -> ${pointerSummary(retval)}`);
      if (this.caller === '0x361b758') {
        console.log(`[UserDB] TaskAwaiter<TResult>.GetResult names ${readStringListListSummary(retval)}`);
      } else if (this.caller === '0x361b8ac') {
        console.log(`[UserDB] TaskAwaiter<TResult>.GetResult whenAll ${readTuple2Array2DSummary(retval)}`);
        const firstInner = readManagedArrayElementPointer(retval, 0);
        console.log(
          `[UserDB] TaskAwaiter<TResult>.GetResult focus ${readTuple2ArrayFocusedTableSummary(firstInner, [
            'IUser',
            'IUserStatus',
            'IUserGem',
            'IUserProfile',
            'IUserLogin',
            'IUserLoginBonus',
            'IUserTutorialProgress',
            'IUserCharacter',
            'IUserCostume',
            'IUserWeapon',
            'IUserCompanion',
            'IUserDeck',
            'IUserDeckCharacter',
            'IUserQuest',
            'IUserMission',
          ])}`,
        );
      } else if (this.caller === '0x3a45c7c') {
        const userDataJson = readObjectPointer(retval, 0x0);
        console.log(
          `[UserDB] TaskAwaiter<TResult>.GetResult response userDataJson=${pointerSummary(userDataJson)} outer=${readListSize(userDataJson)}`,
        );
      }
    },
  });

  hook('UnitySynchronizationContext2.Post(UserData flow)', 0x2e5caf4, {
    onEnter(args) {
      if (!isLikelyUserDataGetFlowCaller(this.returnAddress)) return;
      const callback = args[1];
      const state = args[2];
      const target = callback && !callback.isNull() ? readDelegateTarget(callback) : ptr(0);
      lastUserDataPostCallback = callback;
      lastUserDataPostMethod = callback && !callback.isNull() ? readObjectPointer(callback, 0x18) : ptr(0);
      console.log(
        `[UserDB] UnitySynchronizationContext2.Post req=${activeUserDataRequestId} caller=${moduleOffsetHex(this.returnAddress)} callback=${pointerSummary(callback)} state=${pointerSummary(state)}`,
      );
      if (callback && !callback.isNull()) {
        console.log(`[UserDB] UnitySynchronizationContext2.Post callbackLayout ${readDelegateLayoutSummary(callback)}`);
      }
      if (safeKlassName(target) === 'UserDataGet') {
        console.log(
          `[UserDB] UnitySynchronizationContext2.Post targetFields onSuccess=${pointerSummary(readObjectPointer(target, 0x0))} onError=${pointerSummary(readObjectPointer(target, 0x8))} fetchTable=${pointerSummary(readObjectPointer(target, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(target, 0x18))}`,
        );
      }
    },
  });

  hook('UserDataGet main-thread post callsite', 0x361bd3c, {
    onEnter() {
      const syncContext = this.context.x0;
      const callback = this.context.x1;
      const state = this.context.x2;
      console.log(
        `[UserDB] Post callsite req=${activeUserDataRequestId} syncContext=${pointerSummary(syncContext)} callback=${pointerSummary(callback)} state=${pointerSummary(state)}`,
      );
      if (callback && !callback.isNull()) {
        const method = readObjectPointer(callback, 0x18);
        console.log(
          `[UserDB] Post callsite callbackLayout ${readDelegateLayoutSummary(callback)} methodLoc=${pointerLocationSummary(method)}`,
        );
      }
    },
  });

  hook('SendOrPostCallback.ctor(UserData target)', 0x43ed524, {
    onEnter(args) {
      const self = args[0];
      const target = args[1];
      const method = args[2];
      if (safeKlassName(target) !== 'UserDataGet') return;
      userDataCallbackCtorSeq += 1;
      lastUserDataPostCallback = self;
      lastUserDataPostMethod = method;
      userDataCallbackMethodsSeen.push(method.toString());
      console.log(
        `[UserDB] SendOrPostCallback.ctor req=${activeUserDataRequestId} seq=${userDataCallbackCtorSeq} self=${pointerSummary(self)} target=${pointerSummary(target)} method=${method} methodLoc=${pointerLocationSummary(method)}`,
      );
      console.log(
        `[UserDB] SendOrPostCallback.ctor targetFields onSuccess=${pointerSummary(readObjectPointer(target, 0x0))} onError=${pointerSummary(readObjectPointer(target, 0x8))} fetchTable=${pointerSummary(readObjectPointer(target, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(target, 0x18))}`,
      );
    },
  });

  hook('SendOrPostCallback.Invoke(UserData target)', 0x43e9ae0, {
    onEnter(args) {
      const self = args[0];
      const state = args[1];
      const target = readDelegateTarget(self);
      if (safeKlassName(target) !== 'UserDataGet') return;
      console.log(
        `[UserDB] SendOrPostCallback.Invoke req=${activeUserDataRequestId} self=${pointerSummary(self)} target=${pointerSummary(target)} state=${pointerSummary(state)}`,
      );
      console.log(`[UserDB] SendOrPostCallback.Invoke layout ${readDelegateLayoutSummary(self)}`);
    },
  });

  hook('FetchTargetTableMethod.Invoke', 0x35c761c, {
    onEnter(args) {
      this.self = args[0];
      console.log(`[UserDB] FetchTargetTableMethod.Invoke self=${pointerSummary(args[0])}`);
      console.log(`[UserDB] FetchTargetTableMethod.Invoke layout ${readDelegateLayoutSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(
        `[UserDB] FetchTargetTableMethod.Invoke -> ${pointerSummary(retval)} self=${pointerSummary(this.self)}`,
      );
    },
  });

  hook('FetchRecordMethod.Invoke', 0x35c7240, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] FetchRecordMethod.Invoke self=${pointerSummary(args[0])} tableName=${readManagedString(args[1])}`,
      );
      console.log(`[UserDB] FetchRecordMethod.Invoke layout ${readDelegateLayoutSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(
        `[UserDB] FetchRecordMethod.Invoke -> ${pointerSummary(retval)} self=${pointerSummary(this.self)}`,
      );
    },
  });

  hook('CalculatorNetworking.GetUserDataGetDataSource', 0x2e1bd4c, {
    onEnter(args) {
      console.log(
        `[UserDB] GetUserDataGetDataSource onSuccess=${pointerSummary(args[0])} onError=${pointerSummary(args[1])}`,
      );
      if (args[1] && !args[1].isNull()) {
        console.log(`[UserDB] GetUserDataGetDataSource onErrorLayout ${readDelegateLayoutSummary(args[1])}`);
      }
    },
    onLeave(retval) {
      console.log(`[UserDB] GetUserDataGetDataSource -> ${pointerSummary(retval)}`);
      if (retval && !retval.isNull()) {
        console.log(
          `[UserDB] GetUserDataGetDataSource targetFields onSuccess=${pointerSummary(readObjectPointer(retval, 0x0))} onError=${pointerSummary(readObjectPointer(retval, 0x8))} fetchTable=${pointerSummary(readObjectPointer(retval, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(retval, 0x18))}`,
        );
      }
    },
  });

  hook('CalculatorNetworking.DisposeUserDataGetDataSource', 0x2e1bdec, {
    onEnter(args) {
      console.log(
        `[UserDB] DisposeUserDataGetDataSource dataSource=${pointerSummary(args[0])} onSuccess=${pointerSummary(args[1])} onError=${pointerSummary(args[2])}`,
      );
    },
  });

  hook('UserDataGet.add_OnSuccess', 0x361aa10, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] add_OnSuccess self=${pointerSummary(args[0])} value=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] add_OnSuccess fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))}`,
      );
    },
  });

  hook('UserDataGet.remove_OnSuccess', 0x361aab4, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] remove_OnSuccess self=${pointerSummary(args[0])} value=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] remove_OnSuccess fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))}`,
      );
    },
  });

  hook('UserDataGet.add_OnError', 0x361ab58, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] add_OnError self=${pointerSummary(args[0])} value=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] add_OnError fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))}`,
      );
    },
  });

  hook('UserDataGet.remove_OnError', 0x361abfc, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] remove_OnError self=${pointerSummary(args[0])} value=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] remove_OnError fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))}`,
      );
    },
  });

  hook('UserDataGet.Initialize', 0x361aca0, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] Initialize self=${pointerSummary(args[0])} fetchTableHandler=${pointerSummary(args[1])} fetchRecordHandler=${pointerSummary(args[2])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] Initialize fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))} fetchTable=${pointerSummary(readObjectPointer(this.self, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(this.self, 0x18))}`,
      );
    },
  });

  hook('UserDataGet.InitializeHandler', 0x361aca8, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] InitializeHandler self=${pointerSummary(args[0])} fetchTableHandler=${pointerSummary(args[1])} fetchRecordHandler=${pointerSummary(args[2])}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] InitializeHandler fields onSuccess=${pointerSummary(readObjectPointer(this.self, 0x0))} onError=${pointerSummary(readObjectPointer(this.self, 0x8))} fetchTable=${pointerSummary(readObjectPointer(this.self, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(this.self, 0x18))}`,
      );
    },
  });

  hook('UserDataGet.InitializeDefault', 0x361acb0, {
    onEnter(args) {
      this.self = args[0];
      console.log(`[UserDB] InitializeDefault self=${pointerSummary(args[0])}`);
    },
    onLeave() {
      const onSuccess = readObjectPointer(this.self, 0x0);
      const onError = readObjectPointer(this.self, 0x8);
      const fetchTableHandler = readObjectPointer(this.self, 0x10);
      const fetchRecordHandler = readObjectPointer(this.self, 0x18);
      console.log(
        `[UserDB] InitializeDefault completed self=${pointerSummary(this.self)} onSuccess=${pointerSummary(onSuccess)} onError=${pointerSummary(onError)} fetchTableHandler=${pointerSummary(fetchTableHandler)} fetchRecordHandler=${pointerSummary(fetchRecordHandler)}`,
      );
      if (fetchTableHandler && !fetchTableHandler.isNull()) {
        console.log(`[UserDB] InitializeDefault fetchTableLayout ${readDelegateLayoutSummary(fetchTableHandler)}`);
      }
      if (fetchRecordHandler && !fetchRecordHandler.isNull()) {
        console.log(`[UserDB] InitializeDefault fetchRecordLayout ${readDelegateLayoutSummary(fetchRecordHandler)}`);
      }
    },
  });

  hook('UserDataGet.<RequestAsync>b__0', 0x361b5b0, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__0 req=${activeUserDataRequestId} self=${pointerSummary(args[0])} lastPostedCallback=${pointerSummary(lastUserDataPostCallback)} lastPostedMethod=${lastUserDataPostMethod}`,
      );
      if (lastUserDataPostMethod && !lastUserDataPostMethod.isNull()) {
        console.log(`[UserDB] <RequestAsync>b__0 lastPostedMethodLoc=${pointerLocationSummary(lastUserDataPostMethod)}`);
      }
    },
    onLeave(retval) {
      console.log(`[UserDB] <RequestAsync>b__0 -> ${pointerSummary(retval)}`);
    },
  });

  hook('Task.Run<TResult>(UserData worker)', 0x38ae49c, {
    onEnter(args) {
      const functionObj = args[0];
      const cancellationTokenPtr = args[1];
      const target = functionObj && !functionObj.isNull() ? readDelegateTarget(functionObj) : ptr(0);
      const method = functionObj && !functionObj.isNull() ? readObjectPointer(functionObj, 0x18) : ptr(0);
      const targetName = safeKlassName(target);
      if (targetName !== 'UserDataGet') return;
      this.shouldLog = true;
      console.log(
        `[UserDB] Task.Run<TResult> req=${activeUserDataRequestId} function=${pointerSummary(functionObj)} target=${pointerSummary(target)} cancellationTokenPtr=${pointerSummary(cancellationTokenPtr)}`,
      );
      console.log(
        `[UserDB] Task.Run<TResult> layout ${readDelegateLayoutSummary(functionObj)} methodLoc=${pointerLocationSummary(method)}`,
      );
      console.log(
        `[UserDB] Task.Run<TResult> targetFields onSuccess=${pointerSummary(readObjectPointer(target, 0x0))} onError=${pointerSummary(readObjectPointer(target, 0x8))} fetchTable=${pointerSummary(readObjectPointer(target, 0x10))} fetchRecord=${pointerSummary(readObjectPointer(target, 0x18))}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[UserDB] Task.Run<TResult> -> ${pointerSummary(retval)}`);
    },
  });

  hook('UserDataGet.<RequestAsync>b__11_1', 0x361ae60, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_1 req=${activeUserDataRequestId} self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])} lastPostedCallback=${pointerSummary(lastUserDataPostCallback)} lastPostedMethod=${lastUserDataPostMethod} callbackCtorSeq=${userDataCallbackCtorSeq} callbackMethodsSeen=${userDataCallbackMethodsSeen.join(',')}`,
      );
      if (lastUserDataPostMethod && !lastUserDataPostMethod.isNull()) {
        console.log(`[UserDB] <RequestAsync>b__11_1 lastPostedMethodLoc=${pointerLocationSummary(lastUserDataPostMethod)}`);
      }
    },
    onLeave() {
      console.log('[UserDB] <RequestAsync>b__11_1 completed');
    },
  });

  hook('UserDataGet.<RequestAsync>b__11_3', 0x361b318, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_3 req=${activeUserDataRequestId} self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])} lastPostedCallback=${pointerSummary(lastUserDataPostCallback)} lastPostedMethod=${lastUserDataPostMethod} callbackCtorSeq=${userDataCallbackCtorSeq} callbackMethodsSeen=${userDataCallbackMethodsSeen.join(',')}`,
      );
      if (lastUserDataPostMethod && !lastUserDataPostMethod.isNull()) {
        console.log(`[UserDB] <RequestAsync>b__11_3 lastPostedMethodLoc=${pointerLocationSummary(lastUserDataPostMethod)}`);
      }
    },
    onLeave() {
      console.log('[UserDB] <RequestAsync>b__11_3 completed');
    },
  });

  hook('UserDataGet.HandleError.Invoke', 0x361b328, {
    onEnter(args) {
      console.log(`[UserDB] HandleError.Invoke self=${pointerSummary(args[0])}`);
      console.log(`[UserDB] HandleError.Invoke layout ${readDelegateLayoutSummary(args[0])}`);
    },
  });

  hook('UserDataGet.HandleError.ctor', 0x361be18, {
    onEnter(args) {
      const self = args[0];
      const target = args[1];
      const method = args[2];
      if (safeKlassName(target) !== '<>c__DisplayClass7_0' && safeKlassName(target) !== 'UserDataGet') return;
      console.log(
        `[UserDB] HandleError.ctor req=${activeUserDataRequestId} self=${pointerSummary(self)} target=${pointerSummary(target)} method=${method} methodLoc=${pointerLocationSummary(method)}`,
      );
    },
  });

  hook('UserDataGet.HandleSuccess.Invoke', 0x361af84, {
    onEnter(args) {
      console.log(
        `[UserDB] HandleSuccess.Invoke self=${pointerSummary(args[0])} updatedTableNames=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('UserDataGet.HandleSuccess.ctor', 0x361be60, {
    onEnter(args) {
      const self = args[0];
      const target = args[1];
      const method = args[2];
      if (safeKlassName(target) !== 'UserDataGet' && safeKlassName(target) !== '<>c__DisplayClass7_0') return;
      console.log(
        `[UserDB] HandleSuccess.ctor req=${activeUserDataRequestId} self=${pointerSummary(self)} target=${pointerSummary(target)} method=${method} methodLoc=${pointerLocationSummary(method)}`,
      );
    },
  });

  hook('DarkServerAPI<object, object>.OnErrorRequest', 0x3f7a170, {
    onEnter(args) {
      console.log(
        `[UserDB] DarkServerAPI.OnErrorRequest self=${pointerSummary(args[0])} exception=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('DarkServerAPI.HandleError<object, object>.Invoke', 0x3f795fc, {
    onEnter(args) {
      console.log(
        `[UserDB] DarkServerAPI.HandleError.Invoke self=${pointerSummary(args[0])} error=${readClientErrorSummary(args[1])}`,
      );
    },
  });

  hook('Title.<AddDataSource>b__22_1', 0x31f9854, {
    onEnter(args) {
      console.log(
        `[UserDB] Title.<AddDataSource>b__22_1 self=${pointerSummary(args[0])} error=${readClientErrorSummary(args[1])}`,
      );
    },
  });

  hook('ReviewEnvironmentComposite.<AddDataSource>b__4_1', 0x3f7a3a4, {
    onEnter(args) {
      console.log(
        `[UserDB] ReviewEnvironmentComposite.<AddDataSource>b__4_1 self=${pointerSummary(args[0])} error=${readClientErrorSummary(args[1])}`,
      );
    },
  });

  hook('UserAuthComposite.<AddDataSource>b__4_1', 0x3f7a9f8, {
    onEnter(args) {
      console.log(
        `[UserDB] UserAuthComposite.<AddDataSource>b__4_1 self=${pointerSummary(args[0])} error=${readClientErrorSummary(args[1])}`,
      );
    },
  });

  hook('Title.<SyncUserData>b__0', 0x30bf130, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[UserDB] Title.<SyncUserData>b__0 self=${pointerSummary(args[0])} isErrorBefore=${readObjectBool(args[0], 0x0)}`,
      );
    },
    onLeave() {
      console.log(
        `[UserDB] Title.<SyncUserData>b__0 completed isErrorAfter=${readObjectBool(this.self, 0x0)}`,
      );
    },
  });

  hook('Title.<SyncUserData>d__7.MoveNext', 0x29334d0, {
    onEnter(args) {
      this.self = args[0];
      const state = readRawInt32(args[0], 0x0);
      const displayClass = readRawPointer(args[0], 0x20);
      const isError = displayClass.isNull() ? null : readObjectBool(displayClass, 0x0);
      console.log(
        `[UserDB] Title.<SyncUserData>d__7.MoveNext self=${pointerSummary(args[0])} state=${state} displayClass=${pointerSummary(displayClass)} isError=${isError}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const displayClass = readRawPointer(this.self, 0x20);
      const isError = displayClass.isNull() ? null : readObjectBool(displayClass, 0x0);
      console.log(
        `[UserDB] Title.<SyncUserData>d__7.MoveNext completed self=${pointerSummary(this.self)} state=${state} displayClass=${pointerSummary(displayClass)} isError=${isError}`,
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
      const state = readRawInt32(args[0], 0x0);
      const tableNames = readRawPointer(args[0], 0x20);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__1.MoveNext self=${pointerSummary(args[0])} state=${state} tableNames=${pointerSummary(tableNames)}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const tableNames = readRawPointer(this.self, 0x20);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__1.MoveNext completed self=${pointerSummary(this.self)} state=${state} tableNames=${pointerSummary(tableNames)}`,
      );
    },
  });

  hook('GetUserDataApi.<RequestAsyncMethod>d__0.MoveNext', 0x3a456b8, {
    onEnter(args) {
      this.self = args[0];
      const state = readRawInt32(args[0], 0x0);
      const tableName = readRawPointer(args[0], 0x20);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__0.MoveNext self=${pointerSummary(args[0])} state=${state} tableName=${pointerSummary(tableName)}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const tableName = readRawPointer(this.self, 0x20);
      console.log(
        `[UserAPI] <RequestAsyncMethod>d__0.MoveNext completed self=${pointerSummary(this.self)} state=${state} tableName=${pointerSummary(tableName)}`,
      );
    },
  });

  hook('UserDataGet.<RequestAsync>d__11.MoveNext', 0x361b624, {
    onEnter(args) {
      this.self = args[0];
      const state = readRawInt32(args[0], 0x0);
      const selfObj = readRawPointer(args[0], 0x20);
      const displayClass = readRawPointer(args[0], 0x28);
      const context = readRawPointer(args[0], 0x30);
      const databaseBuilder = readRawPointer(args[0], 0x38);
      const bin = displayClass.isNull() ? ptr(0) : readObjectPointer(displayClass, 0x0);
      console.log(
        `[UserDB] <RequestAsync>d__11.MoveNext self=${pointerSummary(args[0])} state=${state} this=${pointerSummary(selfObj)} displayClass=${pointerSummary(displayClass)} context=${pointerSummary(context)} databaseBuilder=${pointerSummary(databaseBuilder)} bin=${pointerSummary(bin)} binLen=${readByteArrayLength(bin)}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const selfObj = readRawPointer(this.self, 0x20);
      const displayClass = readRawPointer(this.self, 0x28);
      const context = readRawPointer(this.self, 0x30);
      const databaseBuilder = readRawPointer(this.self, 0x38);
      const bin = displayClass.isNull() ? ptr(0) : readObjectPointer(displayClass, 0x0);
      console.log(
        `[UserDB] <RequestAsync>d__11.MoveNext completed self=${pointerSummary(this.self)} state=${state} this=${pointerSummary(selfObj)} displayClass=${pointerSummary(displayClass)} context=${pointerSummary(context)} databaseBuilder=${pointerSummary(databaseBuilder)} bin=${pointerSummary(bin)} binLen=${readByteArrayLength(bin)}`,
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
