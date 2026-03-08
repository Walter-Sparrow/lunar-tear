# Lunar Tear Progress

Last updated: 2026-03-08

`docs/PROGRESS.md` is deprecated. This file tracks only the current, low-assumption state.

## Goal
Server-first fix for the 40% stall with minimal client patches, ideally only host redirection and temporary Frida diagnostics.

## Trusted Current State
- Asset serving works with revision `0` fallback.
- Master data download and parse complete.
- The foreground server receives the real client gRPC traffic.
- `GetUserDataNameV2` and `GetUserData` both complete at the transport level.
- The runtime `GetUserData` path uses plain JSON object arrays, not the old base64+MessagePack experiment.
- The runtime APK diverges from the checked-in source around `CalculatorNetworking` and `UserDataGet`; dump/runtime evidence is more trustworthy than source here.
- An all-empty user-data response takes the healthy path:
  - `DatabaseBuilderBase.Build`
  - non-null `bin`
  - `DarkUserMemoryDatabase.ctor`
  - `UserDataGet.<RequestAsync>b__11_1`
- A response with `IUser=1` is enough to reproduce the failing path even when the other user tables are empty.
- The field-by-field `IUser` search already tried the obvious suspects without changing the branch:
  - `PlayerId`
  - `OsType`
  - `PlatformType`
  - `RegisterDatetime`
  - `GameStartDatetime`
- The assembly boundary is now narrow:
  - `UserDataGet.<RequestAsync>d__11.MoveNext` reaches the tuple loop
  - the hot call at `0x361B930` resolves to `DarkUserDataDatabaseBuilderAppendHelper.Append(...)`
  - failing runs never reach the later `DatabaseBuilderBase.Build()` call at `0x361B968`
- The latest safe runtime slice still shows:
  - request/await path succeeds
  - error callback path ends in `UserDataGet.<RequestAsync>b__11_3`
  - title flips `isError` to `true`

## Current Blocker
The failure is now best described as:
- payload-dependent
- isolated to the `IUser` conversion/append path
- after `GetUserData` transport succeeds
- before `DatabaseBuilderBase.Build()` runs

The strongest current hypothesis is that the runtime throws a managed exception while converting the `IUser` dictionary row into the real runtime entity.

Relevant runtime facts:
- The runtime `EntityIUser` in `dump.cs` uses `MPDateTime` for `RegisterDatetime` and `GameStartDatetime`.
- The runtime also has a real `EntityIUser.ctor(Dictionary<string, object>)` path.
- A direct hook on that ctor was too invasive and caused a crash, so we backed off to safer probes.

## Current Instrumentation
The Frida script is now intentionally trimmed to a minimal signal set.

Enabled logs:
- `il2cpp_raise_exception` only when the backtrace touches the `UserDataGet` / append / `EntityIUser` / `MPDateTime` window
- `UserDataGet.RequestAsync`
- `UserDataGet.<RequestAsync>b__11_1`
- `UserDataGet.<RequestAsync>b__11_3`
- `MPDateTime.ConvertMPDateTime` only when called from the runtime `EntityIUser` ctor window
- `DatabaseBuilderBase.Build`
- `DarkUserMemoryDatabase.ctor`
- `DatabaseDefine.set_User`

Disabled as obsolete/noisy:
- transport and callback correlation spam
- broad awaiter tracing
- legacy datasource wiring logs
- disassembly dumps
- old master-data tracing
- error-dialog and asset retry noise

The old hooks are still kept in the file behind `ENABLE_LEGACY_VERBOSE_HOOKS = false` in case they are needed later.

## Immediate Next Step
Run with the reduced hook set and capture the whole Frida log.

The highest-value lines are now:
- `[UserDB] il2cpp_raise_exception ...`
- `[UserDB] MPDateTime.ConvertMPDateTime ...`
- `[UserDB] <RequestAsync>b__11_3 ...`
- and, if the run succeeds further, `Build` / `ctor` / `set_User`

## Working Theory
If the next exception is:
- `InvalidCastException`: the `IUser` JSON value shape is wrong for runtime conversion
- `KeyNotFoundException`: a required dictionary key is missing or differently named
- `NullReferenceException`: one converted field is accepted structurally but then dereferenced as null
- a date/time-related failure near `MPDateTime.ConvertMPDateTime`: the timestamp representation is still wrong for runtime expectations
