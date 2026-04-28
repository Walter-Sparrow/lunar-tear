# Changelog

## 2026-04-25

### Working

- SQLite persistence layer with snapshot import tool
- Authentication server with login UI
- Wizard CLI for guided first-time setup
- Dev runner (`make dev`) with automatic service builds
- Memoir sub-status system with level-based unlocks
- Companion and parts granting from the shop
- `CopyDeck` / `RemoveDeck` deck management
- Karma functionality
- Docker multi-service orchestration (auth, CDN, gRPC) with cross-platform improvements
- `--grpc-port` CLI flag

### Fixed

- Gate desync on quest-finish crash — scene now advances atomically
- Equipment duplication in deck management

## 2026-04-18

### Working

- Weapon awakening
- Consumable item selling
- `--latest-scene` CLI flag for resuming from the most recent quest scene on startup
- Docker support (Dockerfile, docker-compose, entrypoint)
- GitHub Actions CI for Docker image builds

### Fixed

- Locale fallback MD5 validation for ja/ko -> en asset candidates
- UTF-8 handling for non-ASCII characters in asset path processing
- Tutorial progress no longer overwrites existing progress unless the new phase is greater
- Repeated weapon story unlock notifications — diffs now only send changed stories
- Unique key generation for weapon grants to prevent overwrites in gacha/rewards
- Missing `IUserWeaponStory` in `startedGameStartTables`
- Max level evaluation in costume `EnhanceActiveSkill`

## 2026-04-11

### Working

- Memoir enhancement and deck/memoir management updates
- Companion enhancement
- Costume awakening
- Costume ascending
- Character exalt
- Costume skills level up
- Weapon ascending
- Weapon evolution
- Weapon skills level up
- Quest skipping and auto sale settings
- Item shop
- Deck skins
- [MVP] Gacha system
- [MVP] EX Chapter Quests
- [MVP] Subjugation Battles

### Fixed

- Retire navigation
- Scene transitions mid new arcs

## 2026-04-04

### Working

- Weapon management (enhancement with material consumption, skill/ability tracking, protect/unprotect)
- Mythic slab / character board (panel releases, status effects, ability tracking)
- Explore system
- In-app purchase flow
- Friend service stub
- Master data tooling
- Costume max-level capping by rarity in quest reward flow

### Fixed

- Map freeze caused by gimmick schedule overflow — capped patched entries under the client's MaxGimmickSequenceSchedule=1024 limit

### Roadblock

- Retire quest/battle mechanism — still untraced for quest/battle
- Chapter transition loop — re-login after chapter 7 replays scene 261 instead of advancing

### Need to Figure Out

- Banner/gacha logic (scheduling, rates, pity, relationship between MomBanner and gacha catalogs)

## 2026-03-28

### Working

- Everything from 2026-03-21, plus:
- Costume enhancement (gold cost, material consumption, same-weapon-type EXP bonus)
- Shop (buying items, price deduction, starter item grants on new accounts)
- Mission progress tracking
- 3D viewer
- Event quests (start/finish/restart/update lifecycle, state tracking)
- Tutorial rewards with companion choices
- Battle drop rewards on quest finish
- Snapshot system for saving/loading user state per quest scene

### Roadblock

- Retire quest/battle mechanism — the abandon/withdraw flow for quests and battles hasn't been traced or implemented yet

### Need to Figure Out

- Rarity/awakening, and full client expectations (enhancement is done, rest TBD)
- Banner/gacha logic (scheduling, rates, pity, relationship between MomBanner and gacha catalogs)

## 2026-03-21

### Working

- Login and account creation flow (ToS, name entry, graphic settings, title completion)
- Deck configuration
- Cage ornament rewards
- Main quest progression up to the first battle-only quest obstacle

### Roadblock

- Battle-only quests — the quest engine handles story-driven quests but pure battle encounters use a different entry/exit path that hasn't been traced yet

### Need to Figure Out

- How costumes work in-game (equip rules, stats, rarity/awakening, client expectations)
- Banner/gacha logic (scheduling, rates, pity, relationship between MomBanner and gacha catalogs)
