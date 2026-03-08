'use strict';

// Focused UserDataGet probe.
//
// Keeps the proven loader style from the working hook file, but only logs:
// - request start/finish
// - awaited GetUserData results
// - databaseBuilder/bin state
// - early IUser-specific helpers if they fire
// - failure vs success branch
// - build/publication checkpoints

let libil2cpp;
let userDataRequestSeq = 0;
let activeUserDataRequestId = 0;

function awaitLibil2cpp(callback) {
  if (globalThis._userDataFocusHooksInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._userDataFocusHooksInstalled = true;
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

function readObjectPointer(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return ptr(0);
    return obj.add(0x10 + fieldOffset).readPointer();
  } catch (error) {
    return ptr(0);
  }
}

function readObjectInt64(obj, fieldOffset) {
  try {
    if (!obj || obj.isNull()) return null;
    return obj.add(0x10 + fieldOffset).readS64().toString();
  } catch (error) {
    return null;
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
    return `0x${addr.sub(libil2cpp).toString(16)}`;
  } catch (error) {
    return '<err>';
  }
}

function pointerOffsetHex(addr) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return '<null>';
    return `0x${addr.sub(libil2cpp).toString(16)}`;
  } catch (error) {
    return '<err>';
  }
}

function isOffsetInRange(addr, startOffset, endOffsetExclusive) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return false;
    const offset = addr.sub(libil2cpp).toUInt32();
    return offset >= startOffset && offset < endOffsetExclusive;
  } catch (error) {
    return false;
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

function isLikelyTitlePipelineCaller(addr) {
  try {
    if (!addr || addr.isNull() || !libil2cpp) return false;
    const offset = addr.sub(libil2cpp).toUInt32();
    return (
      (offset >= 0x292f898 && offset <= 0x292feb4) ||
      (offset >= 0x29334d0 && offset <= 0x293398c) ||
      (offset >= 0x30be5e8 && offset <= 0x30be690)
    );
  } catch (error) {
    return false;
  }
}

function readManagedScalarSummary(obj) {
  try {
    if (!obj || obj.isNull()) return '<null>';
    const klass = safeKlassName(obj);
    if (klass === 'String') return `"${readManagedString(obj)}"`;
    if (klass === 'Int32') return `${obj.add(0x10).readS32()}`;
    if (klass === 'Int64') return obj.add(0x10).readS64().toString();
    if (klass === 'Boolean') return obj.add(0x10).readU8() !== 0 ? 'true' : 'false';
    if (klass === 'MPDateTime') return `MPDateTime(${readObjectInt64(obj, 0x0)})`;
    return pointerSummary(obj);
  } catch (error) {
    return '<scalar-err>';
  }
}

function readMPDateTimeSummary(obj) {
  try {
    if (!obj || obj.isNull()) return '<null>';
    return `unix=${readObjectInt64(obj, 0x0)}`;
  } catch (error) {
    return '<mpdatetime-err>';
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

function isInterestingUserDataExceptionFrame(addr) {
  return (
    isLikelyUserDataGetFlowCaller(addr) ||
    isOffsetInRange(addr, 0x28f06c8, 0x28f0860) ||
    isOffsetInRange(addr, 0x2f49158, 0x2f49424) ||
    isOffsetInRange(addr, 0x4480a30, 0x4480abc)
  );
}

function getInterestingUserDataExceptionTrace(context, limit = 10) {
  try {
    const frames = Thread.backtrace(context, Backtracer.ACCURATE);
    const relevant = frames.find((addr) => isInterestingUserDataExceptionFrame(addr));
    if (!relevant) return null;
    return {
      relevant,
      summary: frames.slice(0, limit).map((addr) => pointerLocationSummary(addr)).join(' <- '),
    };
  } catch (error) {
    return null;
  }
}

function resolveBranchTarget(branchOffset) {
  const branchAddr = libil2cpp.add(branchOffset);
  const insn = Instruction.parse(branchAddr);
  const match = insn.opStr.match(/#?(0x[0-9a-fA-F]+)/);
  if (!match) {
    throw new Error(`could not parse branch target for 0x${branchOffset.toString(16)}: ${insn.mnemonic} ${insn.opStr}`);
  }
  return ptr(match[1]);
}

awaitLibil2cpp(() => {
  try {
    const raiseException = typeof Module.findExportByName === 'function'
      ? Module.findExportByName('libil2cpp.so', 'il2cpp_raise_exception')
      : null;
    if (raiseException) {
      Interceptor.attach(raiseException, {
        onEnter(args) {
          const trace = getInterestingUserDataExceptionTrace(this.context);
          if (!trace) return;
          console.log(
            `[UserDB] il2cpp_raise_exception ex=${pointerSummary(args[0])} type=${safeKlassName(args[0])} relevant=${pointerOffsetHex(trace.relevant)} trace=${trace.summary}`,
          );
        },
      });
      console.log(`[*] Hook il2cpp_raise_exception @ ${raiseException}`);
    }
  } catch (error) {
    console.log(`[*] il2cpp_raise_exception hook failed: ${error}`);
  }

  hook('UserDataGet.RequestAsync', 0x361ad5c, {
    onEnter(args) {
      userDataRequestSeq += 1;
      activeUserDataRequestId = userDataRequestSeq;
      console.log(`[UserDB] RequestAsync req=${activeUserDataRequestId} self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[UserDB] RequestAsync req=${activeUserDataRequestId} -> ${pointerSummary(retval)}`);
    },
  });

  hook('TaskAwaiter<TResult>.GetResult(UserData pipeline)', 0x4743464, {
    onEnter(args) {
      if (!isLikelyUserDataPipelineCaller(this.returnAddress) && !isLikelyTitlePipelineCaller(this.returnAddress)) return;
      this.shouldLog = true;
      this.caller = moduleOffsetHex(this.returnAddress);
      const task = readRawPointer(args[0], 0x0);
      console.log(
        `[Flow] TaskAwaiter<TResult>.GetResult caller=${this.caller} awaiter=${args[0]} task=${pointerSummary(task)}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[Flow] TaskAwaiter<TResult>.GetResult -> ${pointerSummary(retval)}`);
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
      } else if (isLikelyTitlePipelineCaller(libil2cpp.add(parseInt(this.caller, 16)))) {
        console.log(`[Title] TaskAwaiter<TResult>.GetResult caller=${this.caller} result=${pointerSummary(retval)}`);
      }
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

  try {
    hook('EntityIUser.CreatePrimaryKey', 0x2f49158, {
      onEnter(args) {
        this.source = args[0];
        console.log(`[UserDB] EntityIUser.CreatePrimaryKey source=${pointerSummary(args[0])}`);
      },
      onLeave(retval) {
        console.log(
          `[UserDB] EntityIUser.CreatePrimaryKey -> ${retval.toString()} source=${pointerSummary(this.source)}`,
        );
      },
    });
  } catch (error) {
    console.log(`[*] EntityIUser.CreatePrimaryKey hook failed: ${error}`);
  }

  try {
    hook('MPDateTime.ConvertMPDateTime', 0x4480a30, {
      onEnter(args) {
        const caller = this.returnAddress;
        if (!isOffsetInRange(caller, 0x2f49270, 0x2f49424)) {
          this.skip = true;
          return;
        }
        this.skip = false;
        console.log(
          `[UserDB] MPDateTime.ConvertMPDateTime caller=${pointerOffsetHex(caller)} input=${readManagedScalarSummary(args[0])}`,
        );
      },
      onLeave(retval) {
        if (this.skip) return;
        console.log(`[UserDB] MPDateTime.ConvertMPDateTime -> ${readMPDateTimeSummary(retval)}`);
      },
    });
  } catch (error) {
    console.log(`[*] MPDateTime.ConvertMPDateTime hook failed: ${error}`);
  }

  hook('UserDataGet.<RequestAsync>b__11_1', 0x361ae60, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_1 req=${activeUserDataRequestId} self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('Title.<SyncUserData>b__0', 0x30bf130, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[Title] <SyncUserData>b__0 self=${pointerSummary(args[0])} isErrorBefore=${readObjectBool(args[0], 0x0)}`,
      );
    },
    onLeave() {
      console.log(
        `[Title] <SyncUserData>b__0 completed isErrorAfter=${readObjectBool(this.self, 0x0)}`,
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
        `[Title] <SyncUserData>d__7.MoveNext self=${pointerSummary(args[0])} state=${state} displayClass=${pointerSummary(displayClass)} isError=${isError}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const displayClass = readRawPointer(this.self, 0x20);
      const isError = displayClass.isNull() ? null : readObjectBool(displayClass, 0x0);
      console.log(
        `[Title] <SyncUserData>d__7.MoveNext completed self=${pointerSummary(this.self)} state=${state} displayClass=${pointerSummary(displayClass)} isError=${isError}`,
      );
    },
  });

  hook('Title.IsNeedGameStartApi', 0x30be14c, {
    onEnter(args) {
      this.self = args[0];
      console.log(`[Title] IsNeedGameStartApi self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      console.log(`[Title] IsNeedGameStartApi -> ${retval.toInt32()}`);
    },
  });

  hook('Title.<OnTitleScreen>d__44.MoveNext', 0x292f898, {
    onEnter(args) {
      this.self = args[0];
      const state = readRawInt32(args[0], 0x0);
      const titleSelf = readRawPointer(args[0], 0x18);
      console.log(
        `[Title] <OnTitleScreen>d__44.MoveNext self=${pointerSummary(args[0])} state=${state} title=${pointerSummary(titleSelf)}`,
      );
    },
    onLeave() {
      const state = readRawInt32(this.self, 0x0);
      const titleSelf = readRawPointer(this.self, 0x18);
      console.log(
        `[Title] <OnTitleScreen>d__44.MoveNext completed self=${pointerSummary(this.self)} state=${state} title=${pointerSummary(titleSelf)}`,
      );
    },
  });

  hook('IUserService.GameStartAsync', 0x27e58d8, {
    onEnter(args) {
      console.log(
        `[Title] IUserService.GameStartAsync self=${pointerSummary(args[0])} request=${pointerSummary(args[1])}`,
      );
    },
    onLeave(retval) {
      console.log(`[Title] IUserService.GameStartAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('UserDataGet.<RequestAsync>b__11_3', 0x361b318, {
    onEnter(args) {
      console.log(
        `[UserDB] <RequestAsync>b__11_3 req=${activeUserDataRequestId} self=${pointerSummary(args[0])} stateArg=${pointerSummary(args[1])}`,
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
});
