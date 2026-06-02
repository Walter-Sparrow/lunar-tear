# Master-data bin tools (event / quest management)

The game server loads **only** the encrypted MasterMemory bin at
`server/assets/release/*.bin.e`. The decoded JSON under `server/assets/masterdata/`
is ignored for content. So enabling/disabling event quests and summon banners
means editing that bin: decrypt (AES-256-CBC) → walk the msgpack table-of-contents
→ mutate the int64 datetime columns in place (preserving each table's LZ4 ext
framing) → re-encrypt.

These are the standalone CLI tools. The lunar-base **Admin → Events** page does the
same thing through a UI (it ports this machinery into `web/services/masterdata_bin.py`).

> After patching, the game **client** must re-download master data — fully relaunch
> the app. Restarting the server alone is not enough: the events/summon menus are
> built client-side from the client's copy of the bin. (Repacking changes the bin's
> mtime, which bumps the reported master-data version, so the client re-fetches.)

## Requirements

```
pip install pycryptodome msgpack lz4
```

## Tools

Run from the repo root so the default `--input server/assets/release/<...>.bin.e`
resolves (or pass `--input` explicitly). Always back up the bin first.

- **`patch_masterdata.py`** — the engine + the full content-extension patch
  (bumps expired `EndDatetime`s to 2030, pulls parked banner starts back, dedups
  campaigns, etc.). Imported by the two scripts below for its decrypt / msgpack
  walker / encrypt machinery.

- **`deactivate_event_quests.py --keep <chapterId...>`** — make only the listed
  event-quest chapters active (`m_event_quest_chapter`); push every other chapter's
  `EndDatetime` into the past.
  ```
  python masterdata-tools/deactivate_event_quests.py --keep 500
  ```

- **`deactivate_summons.py --keep <momBannerId...>`** — same for gacha summon
  banners (`m_mom_banner`); empty `--keep` deactivates them all.
  ```
  python masterdata-tools/deactivate_summons.py --keep 8 31
  ```

## Column reference

- `m_event_quest_chapter`: id=0, StartDatetime=8, EndDatetime=9
- `m_mom_banner`: id=0, StartDatetime=6, EndDatetime=7
