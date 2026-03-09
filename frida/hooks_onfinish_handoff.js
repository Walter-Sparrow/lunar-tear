'use strict';

// Focused post-Title.OnFinish investigation.
//
// Source of truth for RVAs and async type names:
// - client/dump_output/dump.cs
//
// Dump-backed methods used here:
// - Title.OnComplete                          0x30BDB44
// - Title.OnFinish                            0x30BDB98
// - Title.set_IsCompleted                     0x30BE698
// - Gameplay.CreateAsyncTitleEndContents      0x2769294
// - Gameplay.StartGameplayStateMachine        0x276CD90
// - Gameplay.OnMainStoryAsync                 0x276CDEC
// - Gameplay.OnRunApplicationAsync            0x276CF4C
// - Gameplay.WaitInitializedScene             0x276CFFC
// - Gameplay.GetFirstGameplayEvent            0x276D098
// - Gameplay.OnTitleAsync                     0x276D0A0
// - Gameplay.RunTitle                         0x2769BB4
//
// Async state-machine MoveNext RVAs are used only to filter exception traces.
// We do NOT hook MoveNext methods directly.

let libil2cpp;
let lastGameplaySelf = ptr(0);
let logSequence = 0;
let pendingCheckBeforeInvokeDepth = 0;
let trackedCheckBeforeRequest = ptr(0);
let trackedCheckBeforeRequestContext = ptr(0);
let trackedCheckBeforeResponseContext = ptr(0);
let trackedCheckBeforeAsyncUnaryCall = ptr(0);
let trackedCheckBeforeTask = ptr(0);

const OFFSETS = {
  titleOnComplete: 0x30bdb44,
  titleOnFinish: 0x30bdb98,
  titleSetIsCompleted: 0x30be698,
  requestContextCtor: 0x362e4e0,
  gameplayServiceCheckBeforeGamePlayAsync: 0x27e0020,
  darkClientInvokeAsyncCheckBeforeGamePlay: 0x38ac274,
  errorHandlingInterceptorSendAsync: 0x2d94dc4,
  errorHandlingInterceptorErrorHandling: 0x2d94ee8,
  responseContextCtorCheckBeforeGamePlay: 0x42727fc,
  responseContextGetResponseAsCheckBeforeGamePlay: 0x38ab888,
  responseContextGetResponseAsync: 0x4272830,
  responseContextWaitResponseAsync: 0x42728e4,
  responseContextDispose: 0x4272a64,
  responseContextGetTrailers: 0x4272a84,
  asyncUnaryCallGetResponseAsync: 0x47b9238,
  asyncUnaryCallGetTrailers: 0x47b9240,
  asyncUnaryCallDispose: 0x47b9298,
  checkBeforeGamePlayRequestCtor: 0x3e429ec,
  checkBeforeGamePlayRequestGetTr: 0x3e42ae8,
  checkBeforeGamePlayRequestGetVoiceClientSystemLanguageTypeId: 0x3e42b60,
  checkBeforeGamePlayRequestGetTextClientSystemLanguageTypeId: 0x3e42b70,
  checkBeforeGamePlayRequestWriteTo: 0x3e42d5c,
  checkBeforeGamePlayRequestCalculateSize: 0x3e42e14,
  gameplayCreateAsyncTitleEndContents: 0x2769294,
  gameplayStartGameplayStateMachine: 0x276cd90,
  gameplayOnMainStoryAsync: 0x276cdec,
  gameplayOnRunApplicationAsync: 0x276cf4c,
  gameplayWaitInitializedScene: 0x276cffc,
  gameplayGetFirstGameplayEvent: 0x276d098,
  gameplayOnTitleAsync: 0x276d0a0,
  gameplayRunTitle: 0x2769bb4,
  checkBeforeGamePlayResponseCtor: 0x3e4334c,
  checkBeforeGamePlayResponseGetIsExistUnreadPop: 0x3e434e0,
  checkBeforeGamePlayResponseGetMenuGachaBadgeInfo: 0x3e434f4,
  checkBeforeGamePlayResponseGetDiffUserData: 0x3e434fc,
  checkBeforeGamePlayResponseMergeFrom: 0x3e438f0,
};

const RELEVANT_MOVE_NEXT = {
  titleOnFinish: 0x292c10c,
  gameplayCreateAsyncTitleEndContents: 0x28c59ac,
  gameplayRunTitle: 0x278d928,
  gameplayOnMainStoryAsync: 0x28a5cfc,
  gameplayOnRunApplicationAsync: 0x28a64a0,
  gameplayWaitInitializedScene: 0x279272c,
  gameplayOnTitleAsync: 0x28a6954,
};

function awaitLibil2cpp(callback) {
  if (globalThis._onFinishHandoffHooksInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._onFinishHandoffHooksInstalled = true;
    console.log('[*] libil2cpp.so loaded at:', libil2cpp);
    callback();
  } catch (error) {
    setTimeout(() => awaitLibil2cpp(callback), 100);
  }
}

function hook(name, offset, callbacks) {
  Interceptor.attach(libil2cpp.add(offset), callbacks);
  console.log(`[*] Hook ${name} @ 0x${offset.toString(16)}`);
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

function pointerSummary(ptrValue) {
  if (!ptrValue || ptrValue.isNull()) return '<null>';
  return `${ptrValue} klass=${safeKlassName(ptrValue)}`;
}

function readManagedString(addr) {
  if (!addr || addr.isNull()) return '<null>';
  try {
    return addr.add(0x14).readUtf16String();
  } catch (error) {
    return '<err>';
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

function readRawBool(rawPtr, fieldOffset) {
  try {
    if (!rawPtr || rawPtr.isNull()) return null;
    return rawPtr.add(fieldOffset).readU8() !== 0;
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

function moduleOffset(addr) {
  try {
    if (!addr || addr.isNull()) return null;
    return addr.sub(libil2cpp).toUInt32();
  } catch (error) {
    return null;
  }
}

function pointerLocationSummary(addr) {
  const off = moduleOffset(addr);
  if (off === null) return '<null>';
  return `0x${off.toString(16)}`;
}

function gameplayStateName(value) {
  switch (value) {
    case 0: return 'Unknown';
    case 1: return 'DevelopConfig';
    case 2: return 'FirstStep';
    case 3: return 'LockApplication';
    case 4: return 'MainStory';
    case 5: return 'Title';
    case 6: return 'DevelopmentActorViewer';
    case 7: return 'DevelopmentUserInterfaceViewer';
    case 8: return 'DevelopmentMinigameShooting';
    default: return `<unknown:${value}>`;
  }
}

function gameplayEventName(value) {
  switch (value) {
    case 0: return 'Unknown';
    case 1: return 'RunDevelopmentMenu';
    case 2: return 'RunApplication';
    case 3: return 'LockApplication';
    case 4: return 'StartGameplay';
    case 5: return 'StartMainStory';
    case 6: return 'StartDevelopmentActorViewer';
    case 7: return 'StartDevelopmentUserInterfaceViewer';
    case 8: return 'StartDevelopmentMinigameShooting';
    default: return `<unknown:${value}>`;
  }
}

function grpcStatusCodeName(value) {
  switch (value) {
    case 0: return 'OK';
    case 1: return 'Cancelled';
    case 2: return 'Unknown';
    case 3: return 'InvalidArgument';
    case 4: return 'DeadlineExceeded';
    case 5: return 'NotFound';
    case 6: return 'AlreadyExists';
    case 7: return 'PermissionDenied';
    case 8: return 'ResourceExhausted';
    case 9: return 'FailedPrecondition';
    case 10: return 'Aborted';
    case 11: return 'OutOfRange';
    case 12: return 'Unimplemented';
    case 13: return 'Internal';
    case 14: return 'Unavailable';
    case 15: return 'DataLoss';
    case 16: return 'Unauthenticated';
    default: return `<unknown:${value}>`;
  }
}

function metadataCount(metadataPtr) {
  try {
    if (!metadataPtr || metadataPtr.isNull()) return 0;
    const entriesList = readRawPointer(metadataPtr, 0x10);
    if (entriesList.isNull()) return 0;
    return readRawInt32(entriesList, 0x18);
  } catch (error) {
    return null;
  }
}

function quoteLogString(value) {
  if (value === null || value === undefined) return '<null>';
  return JSON.stringify(String(value));
}

function rpcExceptionSummary(rpcExceptionPtr) {
  try {
    if (!rpcExceptionPtr || rpcExceptionPtr.isNull()) return 'rpcException=<null>';
    const statusCode = readRawInt32(rpcExceptionPtr, 0x88);
    const detail = readManagedString(readRawPointer(rpcExceptionPtr, 0x90));
    const trailers = readRawPointer(rpcExceptionPtr, 0x98);
    const trailerCount = metadataCount(trailers);
    return [
      `statusCode=${statusCode}(${grpcStatusCodeName(statusCode)})`,
      `detail=${quoteLogString(detail)}`,
      `trailers=${pointerSummary(trailers)}`,
      `trailerCount=${trailerCount}`,
    ].join(' ');
  } catch (error) {
    return `rpcExceptionSummaryError=${error}`;
  }
}

function snapshotGameplay(self) {
  if (!self || self.isNull()) return 'gameplay=<null>';
  const currentState = readRawInt32(self, 0x10);
  const nextState = readRawInt32(self, 0x14);
  const firstState = readRawInt32(self, 0x18);
  const inUpdate = readRawBool(self, 0x38);
  const doUpdateEvent = readRawBool(self, 0x39);
  const requestUpdateEvent = readRawInt32(self, 0x3c);
  const titlePtr = readRawPointer(self, 0x1e8);
  return [
    `self=${pointerSummary(self)}`,
    `currentState=${currentState}(${gameplayStateName(currentState)})`,
    `nextState=${nextState}(${gameplayStateName(nextState)})`,
    `firstState=${firstState}(${gameplayStateName(firstState)})`,
    `inUpdate=${inUpdate}`,
    `doUpdateEvent=${doUpdateEvent}`,
    `requestUpdateEvent=${requestUpdateEvent}(${gameplayEventName(requestUpdateEvent)})`,
    `title=${pointerSummary(titlePtr)}`,
  ].join(' ');
}

function nextSeq() {
  logSequence += 1;
  return logSequence;
}

function isTrackedCheckBeforeResponseContext(self) {
  return !!self && !self.isNull() && !trackedCheckBeforeResponseContext.isNull() && self.equals(trackedCheckBeforeResponseContext);
}

function isTrackedCheckBeforeAsyncUnaryCall(self) {
  return !!self && !self.isNull() && !trackedCheckBeforeAsyncUnaryCall.isNull() && self.equals(trackedCheckBeforeAsyncUnaryCall);
}

function isTrackedCheckBeforeRequest(self) {
  return !!self && !self.isNull() && !trackedCheckBeforeRequest.isNull() && self.equals(trackedCheckBeforeRequest);
}

function isTrackedCheckBeforeRequestContext(self) {
  return !!self && !self.isNull() && !trackedCheckBeforeRequestContext.isNull() && self.equals(trackedCheckBeforeRequestContext);
}

function resetTrackedCheckBefore() {
  trackedCheckBeforeRequest = ptr(0);
  trackedCheckBeforeRequestContext = ptr(0);
  trackedCheckBeforeResponseContext = ptr(0);
  trackedCheckBeforeAsyncUnaryCall = ptr(0);
  trackedCheckBeforeTask = ptr(0);
}

function onFinishRelatedTrace(context, limit = 10) {
  const trace = relevantBacktraceSummary(context, limit);
  if (trace === null) return null;
  const touchesOnFinish =
    trace.includes(pointerLocationSummary(libil2cpp.add(RELEVANT_MOVE_NEXT.titleOnFinish))) ||
    trace.includes(pointerLocationSummary(libil2cpp.add(OFFSETS.titleOnFinish))) ||
    trace.includes(pointerLocationSummary(libil2cpp.add(OFFSETS.gameplayOnTitleAsync))) ||
    trace.includes(pointerLocationSummary(libil2cpp.add(OFFSETS.gameplayOnRunApplicationAsync)));
  return touchesOnFinish ? trace : null;
}

function isRelevantOffset(offset) {
  if (offset === null) return false;
  return Object.values(RELEVANT_MOVE_NEXT).some((value) => Math.abs(offset - value) <= 0x40)
    || Object.values(OFFSETS).some((value) => Math.abs(offset - value) <= 0x80);
}

function relevantBacktraceSummary(context, limit = 10) {
  try {
    const frames = Thread.backtrace(context, Backtracer.ACCURATE);
    const relevant = frames.some((frame) => isRelevantOffset(moduleOffset(frame)));
    if (!relevant) return null;
    return frames.slice(0, limit).map((frame) => pointerLocationSummary(frame)).join(' <- ');
  } catch (error) {
    return `<bt-err:${error}>`;
  }
}

awaitLibil2cpp(() => {
  console.log('[*] OnFinish handoff investigation: NO MoveNext hooks, NO patches');

  try {
    const raiseException = Process.getModuleByName('libil2cpp.so').findExportByName('il2cpp_raise_exception');
    if (raiseException) {
      Interceptor.attach(raiseException, {
        onEnter(args) {
          const trace = relevantBacktraceSummary(this.context);
          if (!trace) return;
          console.log(
            `[OnFinishFlow] il2cpp_raise_exception ex=${pointerSummary(args[0])} type=${safeKlassName(args[0])} trace=${trace}`,
          );
        },
      });
      console.log(`[*] Hook il2cpp_raise_exception @ ${raiseException}`);
    }
  } catch (error) {
    console.log(`[*] il2cpp_raise_exception hook failed: ${error}`);
  }

  hook('Title.OnComplete', OFFSETS.titleOnComplete, {
    onEnter(args) {
      console.log(`[OnFinishFlow] #${nextSeq()} Title.OnComplete self=${pointerSummary(args[0])}`);
    },
  });

  hook('Title.OnFinish', OFFSETS.titleOnFinish, {
    onEnter(args) {
      resetTrackedCheckBefore();
      console.log(
        `[OnFinishFlow] #${nextSeq()} Title.OnFinish self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Title.OnFinish -> ${pointerSummary(retval)}`);
    },
  });

  hook('RequestContext..ctor', OFFSETS.requestContextCtor, {
    onEnter(args) {
      const path = readManagedString(args[2]);
      if (path !== 'GamePlayService/CheckBeforeGamePlayAsync') return;
      trackedCheckBeforeRequestContext = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} RequestContext..ctor self=${pointerSummary(args[0])} client=${pointerSummary(args[1])} path=${path} request=${pointerSummary(args[3])} headers=${pointerSummary(args[5])} onError=${pointerSummary(args[11])} onVerifyToken=${pointerSummary(args[12])}`,
      );
    },
  });

  hook('IGamePlayService.CheckBeforeGamePlayAsync', OFFSETS.gameplayServiceCheckBeforeGamePlayAsync, {
    onEnter(args) {
      trackedCheckBeforeRequest = args[1];
      console.log(
        `[OnFinishFlow] #${nextSeq()} IGamePlayService.CheckBeforeGamePlayAsync self=${pointerSummary(args[0])} request=${pointerSummary(args[1])}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] IGamePlayService.CheckBeforeGamePlayAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('ErrorHandlingInterceptor.SendAsync', OFFSETS.errorHandlingInterceptorSendAsync, {
    onEnter(args) {
      const context = args[1];
      if (!isTrackedCheckBeforeRequestContext(context)) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ErrorHandlingInterceptor.SendAsync self=${pointerSummary(args[0])} context=${pointerSummary(context)} next=${pointerSummary(args[2])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ErrorHandlingInterceptor.SendAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('ErrorHandlingInterceptor.ErrorHandling', OFFSETS.errorHandlingInterceptorErrorHandling, {
    onEnter(args) {
      if (trackedCheckBeforeRequestContext.isNull()) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ErrorHandlingInterceptor.ErrorHandling self=${pointerSummary(args[0])} rpcException=${pointerSummary(args[1])} type=${safeKlassName(args[1])} ${rpcExceptionSummary(args[1])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ErrorHandlingInterceptor.ErrorHandling -> ${pointerSummary(retval)}`);
    },
  });

  hook('CheckBeforeGamePlayRequest..ctor', OFFSETS.checkBeforeGamePlayRequestCtor, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest..ctor self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('CheckBeforeGamePlayRequest.get_Tr', OFFSETS.checkBeforeGamePlayRequestGetTr, {
    onEnter(args) {
      if (!isTrackedCheckBeforeRequest(args[0])) return;
      this.shouldLog = true;
      console.log(`[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest.get_Tr self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] CheckBeforeGamePlayRequest.get_Tr -> "${readManagedString(retval)}"`);
    },
  });

  hook('CheckBeforeGamePlayRequest.get_VoiceClientSystemLanguageTypeId', OFFSETS.checkBeforeGamePlayRequestGetVoiceClientSystemLanguageTypeId, {
    onEnter(args) {
      if (!isTrackedCheckBeforeRequest(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest.get_VoiceClientSystemLanguageTypeId self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] CheckBeforeGamePlayRequest.get_VoiceClientSystemLanguageTypeId -> ${retval.toInt32()}`);
    },
  });

  hook('CheckBeforeGamePlayRequest.get_TextClientSystemLanguageTypeId', OFFSETS.checkBeforeGamePlayRequestGetTextClientSystemLanguageTypeId, {
    onEnter(args) {
      if (!isTrackedCheckBeforeRequest(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest.get_TextClientSystemLanguageTypeId self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] CheckBeforeGamePlayRequest.get_TextClientSystemLanguageTypeId -> ${retval.toInt32()}`);
    },
  });

  hook('CheckBeforeGamePlayRequest.WriteTo(CodedOutputStream)', OFFSETS.checkBeforeGamePlayRequestWriteTo, {
    onEnter(args) {
      if (!isTrackedCheckBeforeRequest(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest.WriteTo self=${pointerSummary(args[0])} output=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      if (!this.shouldLog) return;
      console.log('[OnFinishFlow] CheckBeforeGamePlayRequest.WriteTo completed');
    },
  });

  hook('CheckBeforeGamePlayRequest.CalculateSize', OFFSETS.checkBeforeGamePlayRequestCalculateSize, {
    onEnter(args) {
      if (!isTrackedCheckBeforeRequest(args[0])) return;
      this.shouldLog = true;
      console.log(`[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayRequest.CalculateSize self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] CheckBeforeGamePlayRequest.CalculateSize -> ${retval.toInt32()}`);
    },
  });

  hook('DarkClient.InvokeAsync<CheckBeforeGamePlayRequest, CheckBeforeGamePlayResponse>', OFFSETS.darkClientInvokeAsyncCheckBeforeGamePlay, {
    onEnter(args) {
      this.path = readManagedString(args[1]);
      this.isTrackedCheckBefore = this.path === 'GamePlayService/CheckBeforeGamePlayAsync';
      if (this.isTrackedCheckBefore) {
        pendingCheckBeforeInvokeDepth += 1;
      }
      console.log(
        `[OnFinishFlow] #${nextSeq()} DarkClient.InvokeAsync<CheckBeforeGamePlayRequest, CheckBeforeGamePlayResponse> self=${pointerSummary(args[0])} path=${this.path} request=${pointerSummary(args[2])} requestMethod=${pointerSummary(args[3])}`,
      );
    },
    onLeave(retval) {
      if (this.isTrackedCheckBefore) {
        trackedCheckBeforeTask = retval;
        pendingCheckBeforeInvokeDepth = Math.max(0, pendingCheckBeforeInvokeDepth - 1);
      }
      console.log(`[OnFinishFlow] DarkClient.InvokeAsync<CheckBeforeGamePlayRequest, CheckBeforeGamePlayResponse> -> ${pointerSummary(retval)}`);
    },
  });

  hook('ResponseContext<CheckBeforeGamePlayResponse>..ctor', OFFSETS.responseContextCtorCheckBeforeGamePlay, {
    onEnter(args) {
      if (pendingCheckBeforeInvokeDepth > 0) {
        trackedCheckBeforeResponseContext = args[0];
        trackedCheckBeforeAsyncUnaryCall = args[1];
      }
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext<CheckBeforeGamePlayResponse>..ctor self=${pointerSummary(args[0])} call=${pointerSummary(args[1])}`,
      );
    },
  });

  hook('ResponseContext.GetResponseAs<CheckBeforeGamePlayResponse>', OFFSETS.responseContextGetResponseAsCheckBeforeGamePlay, {
    onEnter(args) {
      if (!isTrackedCheckBeforeResponseContext(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext.GetResponseAs<CheckBeforeGamePlayResponse> self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ResponseContext.GetResponseAs<CheckBeforeGamePlayResponse> -> ${pointerSummary(retval)}`);
    },
  });

  hook('ResponseContext<T>.get_ResponseAsync', OFFSETS.responseContextGetResponseAsync, {
    onEnter(args) {
      if (!isTrackedCheckBeforeResponseContext(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext<T>.get_ResponseAsync self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ResponseContext<T>.get_ResponseAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('ResponseContext<T>.WaitResponseAsync', OFFSETS.responseContextWaitResponseAsync, {
    onEnter(args) {
      if (!isTrackedCheckBeforeResponseContext(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext<T>.WaitResponseAsync self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ResponseContext<T>.WaitResponseAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('AsyncUnaryCall<T>.get_ResponseAsync', OFFSETS.asyncUnaryCallGetResponseAsync, {
    onEnter(args) {
      if (!isTrackedCheckBeforeAsyncUnaryCall(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} AsyncUnaryCall<T>.get_ResponseAsync self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] AsyncUnaryCall<T>.get_ResponseAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('AsyncUnaryCall<T>.GetTrailers', OFFSETS.asyncUnaryCallGetTrailers, {
    onEnter(args) {
      if (!isTrackedCheckBeforeAsyncUnaryCall(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} AsyncUnaryCall<T>.GetTrailers self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] AsyncUnaryCall<T>.GetTrailers -> ${pointerSummary(retval)}`);
    },
  });

  hook('AsyncUnaryCall<T>.Dispose', OFFSETS.asyncUnaryCallDispose, {
    onEnter(args) {
      if (!isTrackedCheckBeforeAsyncUnaryCall(args[0])) return;
      console.log(
        `[OnFinishFlow] #${nextSeq()} AsyncUnaryCall<T>.Dispose self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('ResponseContext<T>.GetTrailers', OFFSETS.responseContextGetTrailers, {
    onEnter(args) {
      if (!isTrackedCheckBeforeResponseContext(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext<T>.GetTrailers self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[OnFinishFlow] ResponseContext<T>.GetTrailers -> ${pointerSummary(retval)}`);
    },
  });

  hook('ResponseContext<T>.Dispose', OFFSETS.responseContextDispose, {
    onEnter(args) {
      if (!isTrackedCheckBeforeResponseContext(args[0])) return;
      console.log(
        `[OnFinishFlow] #${nextSeq()} ResponseContext<T>.Dispose self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('CheckBeforeGamePlayResponse..ctor', OFFSETS.checkBeforeGamePlayResponseCtor, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayResponse..ctor self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('CheckBeforeGamePlayResponse.MergeFrom(CodedInputStream)', OFFSETS.checkBeforeGamePlayResponseMergeFrom, {
    onEnter(args) {
      this.self = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayResponse.MergeFrom self=${pointerSummary(args[0])} input=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      console.log(`[OnFinishFlow] CheckBeforeGamePlayResponse.MergeFrom completed self=${pointerSummary(this.self)}`);
    },
  });

  hook('CheckBeforeGamePlayResponse.get_IsExistUnreadPop', OFFSETS.checkBeforeGamePlayResponseGetIsExistUnreadPop, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayResponse.get_IsExistUnreadPop self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] CheckBeforeGamePlayResponse.get_IsExistUnreadPop -> ${retval.toInt32()}`);
    },
  });

  hook('CheckBeforeGamePlayResponse.get_MenuGachaBadgeInfo', OFFSETS.checkBeforeGamePlayResponseGetMenuGachaBadgeInfo, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayResponse.get_MenuGachaBadgeInfo self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] CheckBeforeGamePlayResponse.get_MenuGachaBadgeInfo -> ${pointerSummary(retval)}`);
    },
  });

  hook('CheckBeforeGamePlayResponse.get_DiffUserData', OFFSETS.checkBeforeGamePlayResponseGetDiffUserData, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} CheckBeforeGamePlayResponse.get_DiffUserData self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] CheckBeforeGamePlayResponse.get_DiffUserData -> ${pointerSummary(retval)}`);
    },
  });

  hook('Title.set_IsCompleted', OFFSETS.titleSetIsCompleted, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} Title.set_IsCompleted self=${pointerSummary(args[0])} value=${args[1].toInt32()}`,
      );
    },
  });

  hook('Gameplay.StartGameplayStateMachine', OFFSETS.gameplayStartGameplayStateMachine, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(`[OnFinishFlow] #${nextSeq()} Gameplay.StartGameplayStateMachine ${snapshotGameplay(args[0])}`);
    },
  });

  hook('Gameplay.GetFirstGameplayEvent', OFFSETS.gameplayGetFirstGameplayEvent, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(`[OnFinishFlow] #${nextSeq()} Gameplay.GetFirstGameplayEvent enter ${snapshotGameplay(args[0])}`);
    },
    onLeave(retval) {
      const eventValue = retval.toInt32();
      console.log(
        `[OnFinishFlow] Gameplay.GetFirstGameplayEvent -> ${eventValue}(${gameplayEventName(eventValue)}) gameplay=${snapshotGameplay(lastGameplaySelf)}`,
      );
    },
  });

  hook('Gameplay.OnRunApplicationAsync', OFFSETS.gameplayOnRunApplicationAsync, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.OnRunApplicationAsync enter ${snapshotGameplay(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.OnRunApplicationAsync -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });

  hook('Gameplay.WaitInitializedScene', OFFSETS.gameplayWaitInitializedScene, {
    onEnter(args) {
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.WaitInitializedScene enter gameplay=${snapshotGameplay(lastGameplaySelf)} cancellationTokenPtr=${args[1]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.WaitInitializedScene -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });

  hook('Gameplay.OnTitleAsync', OFFSETS.gameplayOnTitleAsync, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.OnTitleAsync enter ${snapshotGameplay(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.OnTitleAsync -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });

  hook('Gameplay.RunTitle', OFFSETS.gameplayRunTitle, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.RunTitle enter ${snapshotGameplay(args[0])} cancellationTokenPtr=${args[1]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.RunTitle -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });

  hook('Gameplay.OnMainStoryAsync', OFFSETS.gameplayOnMainStoryAsync, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.OnMainStoryAsync enter ${snapshotGameplay(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.OnMainStoryAsync -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });

  hook('Gameplay.CreateAsyncTitleEndContents', OFFSETS.gameplayCreateAsyncTitleEndContents, {
    onEnter(args) {
      lastGameplaySelf = args[0];
      console.log(
        `[OnFinishFlow] #${nextSeq()} Gameplay.CreateAsyncTitleEndContents enter ${snapshotGameplay(args[0])} cancellationTokenPtr=${args[1]}`,
      );
    },
    onLeave(retval) {
      console.log(`[OnFinishFlow] Gameplay.CreateAsyncTitleEndContents -> ${pointerSummary(retval)} gameplay=${snapshotGameplay(lastGameplaySelf)}`);
    },
  });
});
