'use strict';

// Focused Story quest-progress probe.
//
// Goal:
// - observe how the client classifies main-quest progress vs terminal state
// - trace the story-side decision chain around scene-progress updates and quest finish
// - inspect the active-player main-quest tables the client reads
//
// Important:
// - no MoveNext hooks
// - no patches/replacements
// - dump-backed RVAs only
//
// Source of truth:
// - client/dump_output/dump.cs

let libil2cpp;
let seq = 0;
let trackedUpdateMainQuestRequest = ptr(0);
let trackedFinishMainQuestRequest = ptr(0);
let latestSceneProgressSceneId = -1;
let latestSceneProgressStoryType = -1;

const OFFSETS = {
  storySendUpdateSceneProgressAsync: 0x27a1dc4,
  storyGetUpdateStoryType: 0x27a1e94,
  storySetLatestUpdateSceneProgressStoryType: 0x27a2600,
  storyNeedsUpdateSceneProgress: 0x27a2614,
  storyOnCompleteStory: 0x27a35e4,
  storyFinishEachStoryTypeQuest: 0x27a3698,
  storyCheckPlayingQuest: 0x27a3788,
  storyConfirmedQuestStartAsync: 0x27a3800,
  storyForcePlayAnyQuestWithCompletionUserData: 0x27a38f8,
  storyForcePlayMainOrSubQuestWithCompletionUserData: 0x27a39e8,
  storyOnPlayLastStory: 0x27a3df0,
  storyIfPlayingQuest: 0x27a3ea4,
  storyNeedsConfirmationQuestRestart: 0x27a3ed4,
  storyCheckConfirmationQuestRestartWithSceneId: 0x27a4030,
  storyRestartQuestAsync: 0x27a4244,
  storyGetCurrentStoryHierarchy: 0x279d184,
  storyGetCurrentGameplayStoryType: 0x279cbec,
  storySetCurrentGameplayStoryType: 0x279cbf4,
  storySetCurrentRouteId: 0x279ccf8,
  storySetCurrentChapterId: 0x279cd08,
  storySetCurrentQuestId: 0x279cd18,
  storySetClearedCurrentQuestId: 0x279cd28,
  storySetCurrentSceneId: 0x279cd3c,
  storySetHeadSceneId: 0x279cd4c,
  storySetQuestIsRunInTheBackground: 0x279cde8,
  storyGetPrevGameplayStoryType: 0x279cd8c,
  storySetPrevGameplayStoryType: 0x279cd94,
  storyApplyClearedCurrentQuestId: 0x279e8ec,
  storyApplyCurrentGameplayStoryType: 0x279f76c,
  storyApplyClearedCurrentChapterIdAndQuestId: 0x279f814,
  storyIsClearedQuestWithQuestId: 0x279f784,
  storyIsNeedsPlaySubQuestWithQuestId: 0x27a5f2c,
  storyTryGetInProgressQuestId: 0x27a644c,
  storyNeedsJumpMomScreenForMainStory: 0x27a6bd8,
  storyOnUpdateSceneProgress: 0x27a6e38,
  storyApplySceneIdForStoryHierarchy: 0x27a6eec,
  storyApplySceneIdForStoryHierarchyWithType: 0x27a6f70,
  storyApplyInQuestLastScene: 0x27a70c8,
  storyStartQuestWithoutAnObelisk: 0x27a7314,
  storyEndQuestWithoutAnObelisk: 0x27a740c,
  storySendFinishMainQuestAsync: 0x27a18dc,
  gameplayWaitCompletionScene: 0x2d64288,

  storyStaticNeedsUpdateSceneProgress: 0x2c563ac,
  storyStaticReturnMainStoryAsync: 0x2c57fe4,
  storyStaticInMainStory: 0x2c583b0,

  activePlayerToUserQuestStatus: 0x2ac2f40,
  activePlayerToEntityMainQuestStatus: 0x2ac30a0,
  activePlayerToEntityPlayingMainQuestStatus: 0x2ac31cc,
  activePlayerToEntityMainQuestFlowStatus: 0x2ac32f8,
  userQuestMissionTryFindUniqueCore: 0x3c01d10,

  networkGetRewardGachaAsync: 0x27dfdf4,
  networkGetHeaderNotificationAsync: 0x27e14c4,
  headerNotificationGetHeaderNotificationDataAsync: 0x306521c,

  updateMainQuestSceneProgressRequestCtor: 0x3f16d70,
  updateMainQuestSceneProgressRequestGetQuestSceneId: 0x3f16e20,
  updateMainQuestSceneProgressRequestWriteTo: 0x3f16f98,
  updateMainQuestSceneProgressRequestCalculateSize: 0x3f16ff0,
  updateMainQuestSceneProgressResponseGetDiffUserData: 0x3f1759c,

  finishMainQuestRequestCtor: 0x3744f4c,
  finishMainQuestRequestGetQuestId: 0x3745070,
  finishMainQuestRequestGetIsRetired: 0x3745080,
  finishMainQuestRequestGetIsMainFlow: 0x3745094,
  finishMainQuestRequestGetIsAnnihilated: 0x37450a8,
  finishMainQuestRequestGetIsAutoOrbit: 0x37450bc,
  finishMainQuestRequestGetStorySkipType: 0x37450d0,
  finishMainQuestRequestGetIsReplayFlow: 0x37450e0,
  finishMainQuestResponseGetDiffUserData: 0x3745fa4,
};

const STORY_TYPE = {
  0: 'Unknown',
  1: 'Main',
  2: 'Sub',
  3: 'Event',
  4: 'Pvp',
  5: 'Content',
  6: 'Extra',
  7: 'BigHunt',
  8: 'Portal',
  9: 'CharacterViewer',
  10: 'SideStory',
};

function nextSeq() {
  seq += 1;
  return seq;
}

function awaitLibil2cpp(callback) {
  if (globalThis._storyProgressProbeInstalled) return;
  try {
    libil2cpp = Process.getModuleByName('libil2cpp.so').base;
    globalThis._storyProgressProbeInstalled = true;
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

function pointerSummary(value) {
  if (!value || value.isNull()) return '<null>';
  return `${value} klass=${safeKlassName(value)}`;
}

function readManagedString(addr) {
  if (!addr || addr.isNull()) return '<null>';
  try {
    return addr.add(0x14).readUtf16String();
  } catch (error) {
    return '<err>';
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

function readRawInt32(rawPtr, fieldOffset) {
  try {
    if (!rawPtr || rawPtr.isNull()) return null;
    return rawPtr.add(fieldOffset).readS32();
  } catch (error) {
    return null;
  }
}

function readRawInt64(rawPtr, fieldOffset) {
  try {
    if (!rawPtr || rawPtr.isNull()) return null;
    return rawPtr.add(fieldOffset).readS64().toString();
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

function storyTypeName(value) {
  return Object.prototype.hasOwnProperty.call(STORY_TYPE, value)
    ? STORY_TYPE[value]
    : `<unknown:${value}>`;
}

function storyInstanceStateSummary(self) {
  if (!self || self.isNull()) return 'story=<null>';
  return [
    `currentGameplayStoryType=${readRawInt32(self, 0x54)}(${storyTypeName(readRawInt32(self, 0x54))})`,
    `currentSeasonId=${readRawInt32(self, 0x5c)}`,
    `currentChapterId=${readRawInt32(self, 0x64)}`,
    `currentQuestId=${readRawInt32(self, 0x68)}`,
    `currentSceneId=${readRawInt32(self, 0x70)}`,
    `prevGameplayStoryType=${readRawInt32(self, 0x8c)}(${storyTypeName(readRawInt32(self, 0x8c))})`,
    `latestUpdateSceneProgressStoryType=${readRawInt32(self, 0x140)}(${storyTypeName(readRawInt32(self, 0x140))})`,
  ].join(' ');
}

function mapFieldCount(mapField) {
  try {
    if (!mapField || mapField.isNull()) return -1;
    const dictionary = readRawPointer(mapField, 0x10);
    if (dictionary.isNull()) return -1;
    return dictionary.add(0x20).readS32();
  } catch (error) {
    return -1;
  }
}

function readArrayLength(arrayPtr) {
  try {
    if (!arrayPtr || arrayPtr.isNull()) return -1;
    return arrayPtr.add(0x18).readS32();
  } catch (error) {
    return -1;
  }
}

function valueTupleLongIntIntSummary(tuplePtr) {
  try {
    if (!tuplePtr || tuplePtr.isNull()) return '<null>';
    const userId = tuplePtr.readS64().toString();
    const questId = tuplePtr.add(0x8).readS32();
    const missionId = tuplePtr.add(0xc).readS32();
    return `userId=${userId} questId=${questId} questMissionId=${missionId}`;
  } catch (error) {
    return '<tuple-err>';
  }
}

function mainQuestStatusSummary(entity) {
  if (!entity || entity.isNull()) return 'mainFlow=<null>';
  return [
    `userId=${readRawInt64(entity, 0x10)}`,
    `routeId=${readRawInt32(entity, 0x18)}`,
    `currentSceneId=${readRawInt32(entity, 0x1c)}`,
    `headSceneId=${readRawInt32(entity, 0x20)}`,
    `isReachedLast=${readRawBool(entity, 0x24)}`,
    `latestVersion=${readRawInt64(entity, 0x28)}`,
  ].join(' ');
}

function playingMainQuestStatusSummary(entity) {
  if (!entity || entity.isNull()) return 'playing=<null>';
  return [
    `userId=${readRawInt64(entity, 0x10)}`,
    `currentSceneId=${readRawInt32(entity, 0x18)}`,
    `headSceneId=${readRawInt32(entity, 0x1c)}`,
    `currentQuestFlowType=${readRawInt32(entity, 0x20)}`,
    `latestVersion=${readRawInt64(entity, 0x28)}`,
  ].join(' ');
}

function flowStatusSummary(entity) {
  if (!entity || entity.isNull()) return 'flow=<null>';
  return [
    `userId=${readRawInt64(entity, 0x10)}`,
    `currentQuestFlowType=${readRawInt32(entity, 0x18)}`,
    `latestVersion=${readRawInt64(entity, 0x20)}`,
  ].join(' ');
}

function isTrackedUpdateMainQuestRequest(self) {
  return !!self && !self.isNull() && !trackedUpdateMainQuestRequest.isNull() && self.equals(trackedUpdateMainQuestRequest);
}

function isTrackedFinishMainQuestRequest(self) {
  return !!self && !self.isNull() && !trackedFinishMainQuestRequest.isNull() && self.equals(trackedFinishMainQuestRequest);
}

function shouldTraceLateStory() {
  return latestSceneProgressSceneId >= 11;
}

awaitLibil2cpp(() => {
  console.log('[*] Story quest-progress probe: direct hooks only, no patches');

  hook('Story.SendUpdateSceneProgressAsync', OFFSETS.storySendUpdateSceneProgressAsync, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} Story.SendUpdateSceneProgressAsync self=${pointerSummary(args[0])} updateQuestId=${args[1].toInt32()} updateSceneId=${args[2].toInt32()} requestReplayed=${args[3].toInt32() !== 0} cancellationTokenPtr=${args[4]}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.SendUpdateSceneProgressAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.GetUpdateStoryType', OFFSETS.storyGetUpdateStoryType, {
    onEnter(args) {
      this.sceneId = args[1].toInt32();
      this.replayed = args[2].toInt32() !== 0;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.GetUpdateStoryType self=${pointerSummary(args[0])} requestSceneId=${this.sceneId} requestReplayed=${this.replayed}`,
      );
    },
    onLeave(retval) {
      const storyType = retval.toInt32();
      console.log(
        `[StoryProbe] Story.GetUpdateStoryType sceneId=${this.sceneId} requestReplayed=${this.replayed} -> ${storyType}(${storyTypeName(storyType)})`,
      );
    },
  });

  hook('Story.SetLatestUpdateSceneProgressStoryType', OFFSETS.storySetLatestUpdateSceneProgressStoryType, {
    onEnter(args) {
      const storyType = args[1].toInt32();
      latestSceneProgressStoryType = storyType;
      latestSceneProgressSceneId = args[2].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.SetLatestUpdateSceneProgressStoryType self=${pointerSummary(args[0])} storyType=${storyType}(${storyTypeName(storyType)}) sceneId=${args[2].toInt32()} isReplayed=${args[3].toInt32() !== 0}`,
      );
    },
  });

  hook('Story.OnCompleteStory', OFFSETS.storyOnCompleteStory, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} Story.OnCompleteStory self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.OnCompleteStory -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.NeedsUpdateSceneProgress(instance)', OFFSETS.storyNeedsUpdateSceneProgress, {
    onEnter(args) {
      this.storyType = args[1].toInt32();
      this.sceneId = args[2].toInt32();
      this.replayed = args[3].toInt32() !== 0;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.NeedsUpdateSceneProgress(instance) self=${pointerSummary(args[0])} storyType=${this.storyType}(${storyTypeName(this.storyType)}) sceneId=${this.sceneId} isReplayed=${this.replayed}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] Story.NeedsUpdateSceneProgress(instance) storyType=${storyTypeName(this.storyType)} sceneId=${this.sceneId} isReplayed=${this.replayed} -> ${retval.toInt32() !== 0}`,
      );
    },
  });

  hook('Story.NeedsUpdateSceneProgress(static)', OFFSETS.storyStaticNeedsUpdateSceneProgress, {
    onEnter() {
      console.log(`[StoryProbe] #${nextSeq()} Story.NeedsUpdateSceneProgress(static)`);
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.NeedsUpdateSceneProgress(static) -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.InMainStory', OFFSETS.storyStaticInMainStory, {
    onEnter() {
      console.log(`[StoryProbe] #${nextSeq()} Story.InMainStory`);
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.InMainStory -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.ReturnMainStoryAsync', OFFSETS.storyStaticReturnMainStoryAsync, {
    onEnter(args) {
      const storyType = args[0].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ReturnMainStoryAsync transitionFrom=${storyType}(${storyTypeName(storyType)}) cancellationTokenPtr=${args[1]}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.ReturnMainStoryAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.FinishEachStoryTypeQuest', OFFSETS.storyFinishEachStoryTypeQuest, {
    onEnter(args) {
      const storyType = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.FinishEachStoryTypeQuest self=${pointerSummary(args[0])} storyType=${storyType}(${storyTypeName(storyType)}) playedChapterId=${args[2].toInt32()} playedQuestId=${args[3].toInt32()} retired=${args[4].toInt32() !== 0} annihilated=${args[5].toInt32() !== 0} cancellationTokenPtr=${args[6]}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Story.FinishEachStoryTypeQuest -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.CheckPlayingQuest', OFFSETS.storyCheckPlayingQuest, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.CheckPlayingQuest self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.CheckPlayingQuest -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.ConfirmedQuestStartAsync', OFFSETS.storyConfirmedQuestStartAsync, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ConfirmedQuestStartAsync self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.ConfirmedQuestStartAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.ForcePlayAnyQuestWithCompletionUserData', OFFSETS.storyForcePlayAnyQuestWithCompletionUserData, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ForcePlayAnyQuestWithCompletionUserData self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.ForcePlayAnyQuestWithCompletionUserData -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.ForcePlayMainOrSubQuestWithCompletionUserData', OFFSETS.storyForcePlayMainOrSubQuestWithCompletionUserData, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ForcePlayMainOrSubQuestWithCompletionUserData self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.ForcePlayMainOrSubQuestWithCompletionUserData -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.OnPlayLastStory', OFFSETS.storyOnPlayLastStory, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.OnPlayLastStory self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.OnPlayLastStory -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.IfPlayingQuest', OFFSETS.storyIfPlayingQuest, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} Story.IfPlayingQuest self=${pointerSummary(args[0])} latestSceneId=${latestSceneProgressSceneId}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.IfPlayingQuest -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.NeedsConfirmationQuestRestart', OFFSETS.storyNeedsConfirmationQuestRestart, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} Story.NeedsConfirmationQuestRestart self=${pointerSummary(args[0])} latestSceneId=${latestSceneProgressSceneId}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.NeedsConfirmationQuestRestart -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.CheckConfirmationQuestRestartWithSceneId', OFFSETS.storyCheckConfirmationQuestRestartWithSceneId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.CheckConfirmationQuestRestartWithSceneId self=${pointerSummary(args[0])} sceneId=${args[1].toInt32()} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.CheckConfirmationQuestRestartWithSceneId -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.RestartQuestAsync', OFFSETS.storyRestartQuestAsync, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.RestartQuestAsync self=${pointerSummary(args[0])} firstStarted=${args[1].toInt32() !== 0} started=${args[2].toInt32() !== 0} cancellationTokenPtr=${args[3]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.RestartQuestAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.GetCurrentStoryHierarchy', OFFSETS.storyGetCurrentStoryHierarchy, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} Story.GetCurrentStoryHierarchy self=${pointerSummary(args[0])} latestSceneId=${latestSceneProgressSceneId}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.GetCurrentStoryHierarchy -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.get_CurrentGameplayStoryType', OFFSETS.storyGetCurrentGameplayStoryType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.get_CurrentGameplayStoryType self=${pointerSummary(args[0])} ${storyInstanceStateSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      const storyType = retval.toInt32();
      console.log(`[StoryProbe] Story.get_CurrentGameplayStoryType -> ${storyType}(${storyTypeName(storyType)})`);
    },
  });

  hook('Story.set_CurrentGameplayStoryType', OFFSETS.storySetCurrentGameplayStoryType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      const storyType = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_CurrentGameplayStoryType self=${pointerSummary(args[0])} value=${storyType}(${storyTypeName(storyType)}) before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_CurrentRouteId', OFFSETS.storySetCurrentRouteId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_CurrentRouteId self=${pointerSummary(args[0])} value=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_CurrentChapterId', OFFSETS.storySetCurrentChapterId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_CurrentChapterId self=${pointerSummary(args[0])} value=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_CurrentQuestId', OFFSETS.storySetCurrentQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_CurrentQuestId self=${pointerSummary(args[0])} value=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_ClearedCurrentQuestId', OFFSETS.storySetClearedCurrentQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_ClearedCurrentQuestId self=${pointerSummary(args[0])} value=${args[1].toInt32() !== 0} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_CurrentSceneId', OFFSETS.storySetCurrentSceneId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_CurrentSceneId self=${pointerSummary(args[0])} value=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_HeadSceneId', OFFSETS.storySetHeadSceneId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_HeadSceneId self=${pointerSummary(args[0])} value=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.set_QuestIsRunInTheBackground', OFFSETS.storySetQuestIsRunInTheBackground, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_QuestIsRunInTheBackground self=${pointerSummary(args[0])} value=${args[1].toInt32() !== 0} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.get_PrevGameplayStoryType', OFFSETS.storyGetPrevGameplayStoryType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.get_PrevGameplayStoryType self=${pointerSummary(args[0])} ${storyInstanceStateSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      const storyType = retval.toInt32();
      console.log(`[StoryProbe] Story.get_PrevGameplayStoryType -> ${storyType}(${storyTypeName(storyType)})`);
    },
  });

  hook('Story.set_PrevGameplayStoryType', OFFSETS.storySetPrevGameplayStoryType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      const storyType = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.set_PrevGameplayStoryType self=${pointerSummary(args[0])} value=${storyType}(${storyTypeName(storyType)}) before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.ApplyCurrentGameplayStoryType', OFFSETS.storyApplyCurrentGameplayStoryType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.self = args[0];
      const storyType = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplyCurrentGameplayStoryType self=${pointerSummary(args[0])} storyType=${storyType}(${storyTypeName(storyType)}) before=${storyInstanceStateSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      if (!this.self || !shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.ApplyCurrentGameplayStoryType done after=${storyInstanceStateSummary(this.self)}`);
    },
  });

  hook('Story.ApplyClearedCurrentQuestId', OFFSETS.storyApplyClearedCurrentQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplyClearedCurrentQuestId self=${pointerSummary(args[0])} questId=${args[1].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.ApplyClearedCurrentChapterIdAndQuestId', OFFSETS.storyApplyClearedCurrentChapterIdAndQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplyClearedCurrentChapterIdAndQuestId self=${pointerSummary(args[0])} questId=${args[1].toInt32()} chapterId=${args[2].toInt32()} before=${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.IsClearedQuestWithQuestId', OFFSETS.storyIsClearedQuestWithQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      this.questId = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.IsClearedQuestWithQuestId self=${pointerSummary(args[0])} questId=${this.questId} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.IsClearedQuestWithQuestId questId=${this.questId} -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.IsNeedsPlaySubQuestWithQuestId', OFFSETS.storyIsNeedsPlaySubQuestWithQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      this.questId = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.IsNeedsPlaySubQuestWithQuestId self=${pointerSummary(args[0])} questId=${this.questId} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.IsNeedsPlaySubQuestWithQuestId questId=${this.questId} -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.TryGetInProgressQuestId', OFFSETS.storyTryGetInProgressQuestId, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      this.outPtr = args[1];
      console.log(
        `[StoryProbe] #${nextSeq()} Story.TryGetInProgressQuestId self=${pointerSummary(args[0])} out=${pointerSummary(args[1])} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      let questId = '<unread>';
      try {
        questId = this.outPtr.isNull() ? '<null-out>' : this.outPtr.readS32().toString();
      } catch (error) {
        questId = '<err>';
      }
      console.log(`[StoryProbe] Story.TryGetInProgressQuestId -> ok=${retval.toInt32() !== 0} questId=${questId}`);
    },
  });

  hook('Story.NeedsJumpMomScreenForMainStory', OFFSETS.storyNeedsJumpMomScreenForMainStory, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} Story.NeedsJumpMomScreenForMainStory self=${pointerSummary(args[0])} latestSceneId=${latestSceneProgressSceneId}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] Story.NeedsJumpMomScreenForMainStory -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('Story.OnUpdateSceneProgress', OFFSETS.storyOnUpdateSceneProgress, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.OnUpdateSceneProgress self=${pointerSummary(args[0])} userdata=${pointerSummary(args[1])} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.OnUpdateSceneProgress -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.ApplyInQuestLastScene', OFFSETS.storyApplyInQuestLastScene, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplyInQuestLastScene self=${pointerSummary(args[0])} latestSceneId=${latestSceneProgressSceneId} ${storyInstanceStateSummary(args[0])}`,
      );
    },
  });

  hook('Story.ApplySceneIdForStoryHierarchy(int)', OFFSETS.storyApplySceneIdForStoryHierarchy, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplySceneIdForStoryHierarchy self=${pointerSummary(args[0])} sceneId=${args[1].toInt32()} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
  });

  hook('Story.ApplySceneIdForStoryHierarchy(storyType, sceneId, updateChapter)', OFFSETS.storyApplySceneIdForStoryHierarchyWithType, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      const storyType = args[1].toInt32();
      console.log(
        `[StoryProbe] #${nextSeq()} Story.ApplySceneIdForStoryHierarchyTyped self=${pointerSummary(args[0])} storyType=${storyType}(${storyTypeName(storyType)}) sceneId=${args[2].toInt32()} updateChapter=${args[3].toInt32()} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
  });

  hook('Story.StartQuestWithoutAnObelisk', OFFSETS.storyStartQuestWithoutAnObelisk, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.StartQuestWithoutAnObelisk self=${pointerSummary(args[0])} targetSceneId=${args[1].toInt32()} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.StartQuestWithoutAnObelisk -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.EndQuestWithoutAnObelisk', OFFSETS.storyEndQuestWithoutAnObelisk, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.EndQuestWithoutAnObelisk self=${pointerSummary(args[0])} nextSceneId=${args[1].toInt32()} cancellationTokenPtr=${args[2]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.EndQuestWithoutAnObelisk -> ${pointerSummary(retval)}`);
    },
  });

  hook('Story.SendFinishMainQuestAsync', OFFSETS.storySendFinishMainQuestAsync, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Story.SendFinishMainQuestAsync self=${pointerSummary(args[0])} retired=${args[1].toInt32() !== 0} annihilated=${args[2].toInt32() !== 0} playedQuestId=${args[3].toInt32()} cancellationTokenPtr=${args[4]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Story.SendFinishMainQuestAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Gameplay.WaitCompletionScene', OFFSETS.gameplayWaitCompletionScene, {
    onEnter(args) {
      if (!shouldTraceLateStory()) return;
      console.log(
        `[StoryProbe] #${nextSeq()} Gameplay.WaitCompletionScene self=${pointerSummary(args[0])} cancellationTokenPtr=${args[1]} latestSceneId=${latestSceneProgressSceneId}`,
      );
    },
    onLeave(retval) {
      if (!shouldTraceLateStory()) return;
      console.log(`[StoryProbe] Gameplay.WaitCompletionScene -> ${pointerSummary(retval)}`);
    },
  });

  hook('Network.IGachaService.GetRewardGachaAsync', OFFSETS.networkGetRewardGachaAsync, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} Network.IGachaService.GetRewardGachaAsync self=${pointerSummary(args[0])} request=${pointerSummary(args[1])} latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Network.IGachaService.GetRewardGachaAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('Network.INotificationService.GetHeaderNotificationAsync', OFFSETS.networkGetHeaderNotificationAsync, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} Network.INotificationService.GetHeaderNotificationAsync self=${pointerSummary(args[0])} request=${pointerSummary(args[1])} latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] Network.INotificationService.GetHeaderNotificationAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('HeaderNotification.GetHeaderNotificationDataAsync', OFFSETS.headerNotificationGetHeaderNotificationDataAsync, {
    onEnter() {
      console.log(
        `[StoryProbe] #${nextSeq()} HeaderNotification.GetHeaderNotificationDataAsync latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)}`,
      );
    },
    onLeave(retval) {
      console.log(`[StoryProbe] HeaderNotification.GetHeaderNotificationDataAsync -> ${pointerSummary(retval)}`);
    },
  });

  hook('UpdateMainQuestSceneProgressRequest..ctor', OFFSETS.updateMainQuestSceneProgressRequestCtor, {
    onEnter(args) {
      trackedUpdateMainQuestRequest = args[0];
      console.log(
        `[StoryProbe] #${nextSeq()} UpdateMainQuestSceneProgressRequest..ctor self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('UpdateMainQuestSceneProgressRequest.get_QuestSceneId', OFFSETS.updateMainQuestSceneProgressRequestGetQuestSceneId, {
    onEnter(args) {
      if (!isTrackedUpdateMainQuestRequest(args[0])) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} UpdateMainQuestSceneProgressRequest.get_QuestSceneId self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] UpdateMainQuestSceneProgressRequest.get_QuestSceneId -> ${retval.toInt32()}`);
    },
  });

  hook('UpdateMainQuestSceneProgressRequest.CalculateSize', OFFSETS.updateMainQuestSceneProgressRequestCalculateSize, {
    onEnter(args) {
      if (!isTrackedUpdateMainQuestRequest(args[0])) return;
      this.shouldLog = true;
      console.log(`[StoryProbe] #${nextSeq()} UpdateMainQuestSceneProgressRequest.CalculateSize self=${pointerSummary(args[0])}`);
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] UpdateMainQuestSceneProgressRequest.CalculateSize -> ${retval.toInt32()}`);
    },
  });

  hook('UpdateMainQuestSceneProgressRequest.WriteTo', OFFSETS.updateMainQuestSceneProgressRequestWriteTo, {
    onEnter(args) {
      if (!isTrackedUpdateMainQuestRequest(args[0])) return;
      this.shouldLog = true;
      console.log(
        `[StoryProbe] #${nextSeq()} UpdateMainQuestSceneProgressRequest.WriteTo self=${pointerSummary(args[0])} output=${pointerSummary(args[1])}`,
      );
    },
    onLeave() {
      if (!this.shouldLog) return;
      console.log('[StoryProbe] UpdateMainQuestSceneProgressRequest.WriteTo completed');
    },
  });

  hook('UpdateMainQuestSceneProgressResponse.get_DiffUserData', OFFSETS.updateMainQuestSceneProgressResponseGetDiffUserData, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} UpdateMainQuestSceneProgressResponse.get_DiffUserData self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] UpdateMainQuestSceneProgressResponse.get_DiffUserData -> ${pointerSummary(retval)} count=${mapFieldCount(retval)}`,
      );
    },
  });

  hook('FinishMainQuestRequest..ctor', OFFSETS.finishMainQuestRequestCtor, {
    onEnter(args) {
      trackedFinishMainQuestRequest = args[0];
      console.log(
        `[StoryProbe] #${nextSeq()} FinishMainQuestRequest..ctor self=${pointerSummary(args[0])}`,
      );
    },
  });

  hook('FinishMainQuestRequest.get_QuestId', OFFSETS.finishMainQuestRequestGetQuestId, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_QuestId -> ${retval.toInt32()}`);
    },
  });

  hook('FinishMainQuestRequest.get_IsRetired', OFFSETS.finishMainQuestRequestGetIsRetired, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_IsRetired -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('FinishMainQuestRequest.get_IsMainFlow', OFFSETS.finishMainQuestRequestGetIsMainFlow, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_IsMainFlow -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('FinishMainQuestRequest.get_IsAnnihilated', OFFSETS.finishMainQuestRequestGetIsAnnihilated, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_IsAnnihilated -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('FinishMainQuestRequest.get_IsAutoOrbit', OFFSETS.finishMainQuestRequestGetIsAutoOrbit, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_IsAutoOrbit -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('FinishMainQuestRequest.get_StorySkipType', OFFSETS.finishMainQuestRequestGetStorySkipType, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_StorySkipType -> ${retval.toInt32()}`);
    },
  });

  hook('FinishMainQuestRequest.get_IsReplayFlow', OFFSETS.finishMainQuestRequestGetIsReplayFlow, {
    onEnter(args) {
      if (!isTrackedFinishMainQuestRequest(args[0])) return;
      this.shouldLog = true;
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      console.log(`[StoryProbe] FinishMainQuestRequest.get_IsReplayFlow -> ${retval.toInt32() !== 0}`);
    },
  });

  hook('FinishMainQuestResponse.get_DiffUserData', OFFSETS.finishMainQuestResponseGetDiffUserData, {
    onEnter(args) {
      console.log(
        `[StoryProbe] #${nextSeq()} FinishMainQuestResponse.get_DiffUserData self=${pointerSummary(args[0])}`,
      );
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] FinishMainQuestResponse.get_DiffUserData -> ${pointerSummary(retval)} count=${mapFieldCount(retval)}`,
      );
    },
  });

  hook('ActivePlayerToUserQuestStatus', OFFSETS.activePlayerToUserQuestStatus, {
    onEnter(args) {
      this.questId = args[0].toInt32();
      this.outPtr = args[1];
      console.log(`[StoryProbe] #${nextSeq()} ActivePlayerToUserQuestStatus questId=${this.questId} out=${pointerSummary(this.outPtr)}`);
    },
    onLeave(retval) {
      let outValue = ptr(0);
      try {
        outValue = this.outPtr.isNull() ? ptr(0) : this.outPtr.readPointer();
      } catch (error) {
        outValue = ptr(0);
      }
      console.log(
        `[StoryProbe] ActivePlayerToUserQuestStatus questId=${this.questId} -> ok=${retval.toInt32() !== 0} entity=${pointerSummary(outValue)}`,
      );
    },
  });

  hook('TableBase<EntityIUserQuestMission>.TryFindUniqueCore<ValueTuple<long,int,int>>', OFFSETS.userQuestMissionTryFindUniqueCore, {
    onEnter(args) {
      if (latestSceneProgressSceneId < 11) return;
      this.shouldLog = true;
      this.outPtr = args[4];
      console.log(
        `[StoryProbe] #${nextSeq()} UserQuestMission.TryFindUniqueCore latestSceneId=${latestSceneProgressSceneId} latestStoryType=${storyTypeName(latestSceneProgressStoryType)} indexLen=${readArrayLength(args[0])} key=${valueTupleLongIntIntSummary(args[3])} out=${pointerSummary(args[4])}`,
      );
    },
    onLeave(retval) {
      if (!this.shouldLog) return;
      let outValue = ptr(0);
      try {
        outValue = this.outPtr.isNull() ? ptr(0) : this.outPtr.readPointer();
      } catch (error) {
        outValue = ptr(0);
      }
      console.log(
        `[StoryProbe] UserQuestMission.TryFindUniqueCore -> ok=${retval.toInt32() !== 0} entity=${pointerSummary(outValue)}`,
      );
    },
  });

  hook('ActivePlayerToEntityMainQuestStatus', OFFSETS.activePlayerToEntityMainQuestStatus, {
    onEnter() {
      console.log(`[StoryProbe] #${nextSeq()} ActivePlayerToEntityMainQuestStatus`);
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] ActivePlayerToEntityMainQuestStatus -> ${pointerSummary(retval)} ${mainQuestStatusSummary(retval)}`,
      );
    },
  });

  hook('ActivePlayerToEntityPlayingMainQuestStatus', OFFSETS.activePlayerToEntityPlayingMainQuestStatus, {
    onEnter() {
      console.log(`[StoryProbe] #${nextSeq()} ActivePlayerToEntityPlayingMainQuestStatus`);
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] ActivePlayerToEntityPlayingMainQuestStatus -> ${pointerSummary(retval)} ${playingMainQuestStatusSummary(retval)}`,
      );
    },
  });

  hook('ActivePlayerToEntityMainQuestFlowStatus', OFFSETS.activePlayerToEntityMainQuestFlowStatus, {
    onEnter() {
      console.log(`[StoryProbe] #${nextSeq()} ActivePlayerToEntityMainQuestFlowStatus`);
    },
    onLeave(retval) {
      console.log(
        `[StoryProbe] ActivePlayerToEntityMainQuestFlowStatus -> ${pointerSummary(retval)} ${flowStatusSummary(retval)}`,
      );
    },
  });
});
