#!/usr/bin/env python3
"""Patch master data timestamps to extend content availability.

Decrypts the MasterMemory binary (.bin.e), extends EndDatetime fields from
the 2020-2029 range to 2030-12-31, and re-encrypts with the same AES key/IV.

Requires: pip install pycryptodome msgpack lz4
"""

import argparse
import os
import struct
import sys
from datetime import datetime, timezone

import lz4.block
import msgpack
from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad


DEFAULT_INPUT = os.path.join("server", "assets", "release", "20240404193219.bin.e")
DEFAULT_KEY = "36436230313332314545356536624265"
DEFAULT_IV  = "45666341656634434165356536446141"

TARGET_END_DT = datetime(2030, 12, 31, 23, 59, 59, tzinfo=timezone.utc)
TARGET_END_MS = int(TARGET_END_DT.timestamp() * 1000)
MIN_PATCH_MS  = int(datetime(2020, 1, 1, tzinfo=timezone.utc).timestamp() * 1000)
MAX_PATCH_MS  = int(datetime(2030, 1, 1, tzinfo=timezone.utc).timestamp() * 1000)

# Content parked far in the future (e.g. banner reruns at 2099-08-19) never
# starts. Pull any StartDatetime later than START_PULLBACK_AFTER_MS back to
# START_PULLBACK_TARGET_MS so it becomes available now.
START_PULLBACK_AFTER_MS  = MAX_PATCH_MS   # start after 2030-01-01 = "parked"
START_PULLBACK_TARGET_MS = MIN_PATCH_MS   # pulled back to 2020-01-01

# Enhance keeps only the universal *_ALL targets ({1,2,3}); everything else
# (per-character, per-costume, per-weapon, per-attribute, per-series, per-id)
# is dropped to avoid extending entity-specific rerun boosts forever.
ENTITY_ID_TARGETS_ENHANCE = frozenset({11, 12, 13, 21, 22, 23, 31, 32})
ENTITY_ID_TARGETS_QUEST   = frozenset({5, 7})  # MAIN_QUEST_QUEST_ID, SUB_QUEST_QUEST_ID

SKIP_TABLES  = frozenset({"m_omikuji"})
EMPTY_TABLES = frozenset({"m_maintenance"})

# Unhide the Labyrinth (EventQuestType=12) in the side-quest menu — the client
# hides every chapter of any EventQuestType missing from m_event_quest_unlock_condition.
TABLE_ROW_ADDITIONS = {
    'm_event_quest_unlock_condition': [
        [12, 0, 0, 1, 21, 0],
    ],
}

# Client cap MaxGimmickSequenceSchedule = 1024. Through 2023-02 there are 1022
# entries, so anything started after that cutoff stays expired.
SCHEDULE_PATCH_CUTOFF_MS = int(datetime(2023, 2, 1, tzinfo=timezone.utc).timestamp() * 1000)
TABLE_PATCH_FILTERS = {
    'm_gimmick_sequence_schedule': (1, SCHEDULE_PATCH_CUTOFF_MS),
}

# Column indices derived from entity class definitions in schemas.json.
# m_enhance_campaign and m_quest_campaign are intentionally absent — they're
# handled by patch_campaign_dedup() instead of the blanket bump.
PATCH_COLUMNS = {
    'm_appeal_dialog': [(5, 'EndDatetime')],
    'm_big_hunt_schedule': [(3, 'ChallengeEndDatetime')],
    'm_cage_ornament': [(2, 'EndDatetime')],
    'm_consumable_item_term': [(2, 'EndDatetime')],
    'm_costume_collection_bonus': [(6, 'EndDatetime')],
    'm_dokan': [(4, 'EndDatetime')],
    'm_event_quest_chapter': [(9, 'EndDatetime')],
    'm_event_quest_daily_group': [(2, 'EndDatetime')],
    'm_event_quest_guerrilla_free_open': [(4, 'EndDatetime')],
    'm_event_quest_limit_content': [(6, 'EndDatetime')],
    'm_event_quest_limit_content_deck_restriction': [(4, 'EndDatetime')],
    'm_gacha_medal': [(4, 'AutoConvertDatetime')],
    'm_gimmick_sequence_schedule': [(2, 'EndDatetime')],
    'm_important_item_effect': [(6, 'EndDatetime')],
    'm_login_bonus': [(5, 'EndDatetime'), (6, 'StampReceiveEndDatetime')],
    'm_mission_pass': [(2, 'EndDatetime')],
    'm_mission_term': [(2, 'EndDatetime')],
    'm_mom_banner': [(7, 'EndDatetime')],
    'm_mom_point_banner': [(4, 'EndDatetime')],
    'm_navi_cut_in': [(4, 'EndDatetime')],
    'm_omikuji': [(2, 'EndDatetime')],
    'm_portal_cage_access_point_function_group_schedule': [(5, 'EndDatetime')],
    'm_possession_acquisition_route': [(7, 'EndDatetime')],
    'm_premium_item': [(3, 'EndDatetime')],
    'm_pvp_season': [(3, 'SeasonEndDatetime')],
    'm_quest_bonus_term_group': [(3, 'EndDatetime')],
    'm_quest_schedule': [(3, 'EndDatetime')],
    'm_shop': [(10, 'EndDatetime')],
    'm_shop_item_cell_term': [(2, 'EndDatetime')],
    'm_tip': [(6, 'EndDatetime')],
    'm_title_flow_movie': [(3, 'EndDatetime')],
    'm_webview_mission': [(5, 'EndDatetime')],
    'm_webview_panel_mission': [(4, 'EndDatetime')],
}

# StartDatetime columns to pull back from the future so parked content (banner
# reruns, etc.) is active now. Column indices from the entity definitions.
PULL_START_COLUMNS = {
    'm_mom_banner': [(6, 'StartDatetime')],
}

# Campaign-family layout for the unified dedup helper.
CAMPAIGN_CFG = {
    'enhance': dict(
        family='enhance',
        target_table='m_enhance_campaign_target_group',
        effect_table=None,
        id_col=0, tg_col=1, eff_type_col=2, eff_val_col=3,
        start_col=4, end_col=5, user_status_col=6,
        entity_id_targets=ENTITY_ID_TARGETS_ENHANCE,
    ),
    'quest': dict(
        family='quest',
        target_table='m_quest_campaign_target_group',
        effect_table='m_quest_campaign_effect_group',
        id_col=0, tg_col=1, eff_group_col=2,
        start_col=3, end_col=4, user_status_col=5,
        entity_id_targets=ENTITY_ID_TARGETS_QUEST,
    ),
}


# --- LZ4/ExtType helpers ---

def read_lz4_ext_header(ext_data):
    """Strip the msgpack int prefix C#'s LZ4MessagePack writes before the LZ4 bytes."""
    tag = ext_data[0]
    if tag == 0xd2: return struct.unpack('>i', ext_data[1:5])[0], ext_data[5:]
    if tag == 0xce: return struct.unpack('>I', ext_data[1:5])[0], ext_data[5:]
    if tag == 0xd1: return struct.unpack('>h', ext_data[1:3])[0], ext_data[3:]
    if tag == 0xcd: return struct.unpack('>H', ext_data[1:3])[0], ext_data[3:]
    if tag <= 0x7f: return tag, ext_data[1:]
    raise ValueError(f"Unexpected msgpack tag 0x{tag:02x} in LZ4 ext header")


def build_lz4_ext_blob(decompressed_data):
    compressed = lz4.block.compress(decompressed_data, store_size=False)
    header = b'\xd2' + struct.pack('>i', len(decompressed_data))
    return msgpack.packb(msgpack.ExtType(99, header + compressed), use_bin_type=True)


# --- Msgpack binary walker ---
# Hand-rolled to support in-place int64 mutation: msgpack.packb re-encodes
# C#'s int64 columns as Python's tighter int encodings, producing byte-different
# blobs the client's schema validator rejects. So writes use this walker;
# read-only inspection can use msgpack.unpackb freely.

def skip_msgpack_value(data, pos):
    tag = data[pos]
    if tag <= 0x7f or tag >= 0xe0: return pos + 1
    if 0xa0 <= tag <= 0xbf:        return pos + 1 + (tag & 0x1f)
    if 0x90 <= tag <= 0x9f:
        n = tag & 0x0f
        p = pos + 1
        for _ in range(n): p = skip_msgpack_value(data, p)
        return p
    if 0x80 <= tag <= 0x8f:
        n = tag & 0x0f
        p = pos + 1
        for _ in range(n * 2): p = skip_msgpack_value(data, p)
        return p
    FIXED = {
        0xc0: 1, 0xc2: 1, 0xc3: 1,
        0xca: 5, 0xcb: 9,
        0xcc: 2, 0xcd: 3, 0xce: 5, 0xcf: 9,
        0xd0: 2, 0xd1: 3, 0xd2: 5, 0xd3: 9,
        0xd4: 3, 0xd5: 4, 0xd6: 6, 0xd7: 10, 0xd8: 18,
    }
    if tag in FIXED: return pos + FIXED[tag]
    LENGTH_PREFIXED = {
        0xc4: (1, 'B'), 0xc5: (2, '>H'), 0xc6: (4, '>I'),
        0xd9: (1, 'B'), 0xda: (2, '>H'), 0xdb: (4, '>I'),
        0xc7: (1, 'B'), 0xc8: (2, '>H'), 0xc9: (4, '>I'),
    }
    if tag in LENGTH_PREFIXED:
        sz_bytes, fmt = LENGTH_PREFIXED[tag]
        n = struct.unpack(fmt, data[pos + 1:pos + 1 + sz_bytes])[0]
        extra = 1 if tag in (0xc7, 0xc8, 0xc9) else 0
        return pos + 1 + sz_bytes + extra + n
    ARRAY_MAP = {0xdc: (2, '>H'), 0xdd: (4, '>I'), 0xde: (2, '>H'), 0xdf: (4, '>I')}
    if tag in ARRAY_MAP:
        sz_bytes, fmt = ARRAY_MAP[tag]
        n = struct.unpack(fmt, data[pos + 1:pos + 1 + sz_bytes])[0]
        items = n * 2 if tag in (0xde, 0xdf) else n
        p = pos + 1 + sz_bytes
        for _ in range(items): p = skip_msgpack_value(data, p)
        return p
    raise ValueError(f"Unknown msgpack tag 0x{tag:02x} at pos {pos}")


def read_array_len(data, pos):
    tag = data[pos]
    if 0x90 <= tag <= 0x9f: return (tag & 0x0f, pos + 1)
    if tag == 0xdc:         return (struct.unpack('>H', data[pos + 1:pos + 3])[0], pos + 3)
    if tag == 0xdd:         return (struct.unpack('>I', data[pos + 1:pos + 5])[0], pos + 5)
    raise ValueError(f"Expected array at pos {pos}, got tag 0x{tag:02x}")


def read_int(data, pos):
    tag = data[pos]
    if tag <= 0x7f: return tag
    if tag == 0xcc: return data[pos + 1]
    if tag == 0xcd: return struct.unpack('>H', data[pos + 1:pos + 3])[0]
    if tag == 0xce: return struct.unpack('>I', data[pos + 1:pos + 5])[0]
    raise ValueError(f"read_int: unexpected tag 0x{tag:02x} at pos {pos}")


# --- Table-blob mutators ---
# Each takes a bytearray of the decompressed table blob and mutates in place.

def add_table_rows(blob, rows, key_col=0):
    """Append rows whose key isn't already present. Idempotent."""
    count, pos = read_array_len(blob, 0)
    existing = set()
    p = pos
    for _ in range(count):
        _, row_pos = read_array_len(blob, p)
        existing.add(read_int(blob, row_pos))
        p = skip_msgpack_value(blob, p)
    to_add = [r for r in rows if r[key_col] not in existing]
    if not to_add:
        return None
    total = count + len(to_add)
    if total <= 0x0f:    header = bytes([0x90 | total])
    elif total <= 0xffff: header = b'\xdc' + struct.pack('>H', total)
    else:                 header = b'\xdd' + struct.pack('>I', total)
    new_rows = b''.join(msgpack.packb(r, use_bin_type=True) for r in to_add)
    return header + bytes(blob[pos:]) + new_rows


def patch_table_blob(blob, col_indices, row_filter=None):
    """Bump int64 datetime columns in col_indices to TARGET_END_MS."""
    row_count, pos = read_array_len(blob, 0)
    patched = skipped = 0
    for _ in range(row_count):
        col_count, p = read_array_len(blob, pos)

        skip_row = False
        if row_filter is not None:
            filter_col, filter_max = row_filter
            fp = p
            for ci in range(min(filter_col + 1, col_count)):
                if ci == filter_col and blob[fp] == 0xd3:
                    val = struct.unpack('>q', blob[fp + 1:fp + 9])[0]
                    if val >= filter_max:
                        skip_row = True
                    break
                fp = skip_msgpack_value(blob, fp)

        if skip_row:
            skipped += 1
            for _ in range(col_count): p = skip_msgpack_value(blob, p)
        else:
            for col_i in range(col_count):
                if col_i in col_indices and blob[p] == 0xd3:
                    val = struct.unpack('>q', blob[p + 1:p + 9])[0]
                    if MIN_PATCH_MS <= val <= MAX_PATCH_MS:
                        struct.pack_into('>q', blob, p + 1, TARGET_END_MS)
                        patched += 1
                p = skip_msgpack_value(blob, p)
        pos = p
    return patched, skipped


def pull_start_blob(blob, col_indices):
    """Pull future StartDatetime columns (> START_PULLBACK_AFTER_MS) back to
    START_PULLBACK_TARGET_MS so parked content becomes active now."""
    row_count, pos = read_array_len(blob, 0)
    patched = 0
    for _ in range(row_count):
        col_count, p = read_array_len(blob, pos)
        for col_i in range(col_count):
            if col_i in col_indices and blob[p] == 0xd3:
                val = struct.unpack('>q', blob[p + 1:p + 9])[0]
                if val > START_PULLBACK_AFTER_MS:
                    struct.pack_into('>q', blob, p + 1, START_PULLBACK_TARGET_MS)
                    patched += 1
            p = skip_msgpack_value(blob, p)
        pos = p
    return patched


def patch_labyrinth_seasons(blob):
    """Leave exactly one within-period season per chapter — extras get End=0.

    The client's TryGetEventQuestLabyrinthWithinPeriod returns true only if
    EXACTLY ONE row passes IsWithinThePeriod; a second within-period row
    flips it false. Highest SeasonNumber per chapter wins.
    Columns: 0 ChapterId, 1 SeasonNumber, 2 StartDatetime, 3 EndDatetime.
    """
    rows = []
    row_count, pos = read_array_len(blob, 0)
    for _ in range(row_count):
        col_count, p = read_array_len(blob, pos)
        cp = p
        col_pos = []
        for _ in range(col_count):
            col_pos.append(cp)
            cp = skip_msgpack_value(blob, cp)
        rows.append((read_int(blob, col_pos[0]), read_int(blob, col_pos[1]),
                     col_pos[2], col_pos[3]))
        pos = cp

    max_season = {}
    for chapter, season, _, _ in rows:
        if season > max_season.get(chapter, -1):
            max_season[chapter] = season

    written = 0
    for chapter, season, start_pos, end_pos in rows:
        if blob[start_pos] != 0xd3 or blob[end_pos] != 0xd3:
            continue
        if season == max_season[chapter]:
            struct.pack_into('>q', blob, start_pos + 1, MIN_PATCH_MS)
            struct.pack_into('>q', blob, end_pos + 1, TARGET_END_MS)
        else:
            struct.pack_into('>q', blob, end_pos + 1, 0)
        written += 1
    return written


def patch_gimmick_sequence_schedules(blob):
    """Keep one active schedule per FirstGimmickSequenceId — extras get End=0.

    Multiple schedules per FirstSeqId render as overlapping GimmickOrnamentCage
    records and cancel each other. Lowest (StartDt, ScheduleId) is canonical
    (matches server's masterdata/gimmick.go dedup pick).
    Columns: 0 ScheduleId, 1 StartDt, 2 EndDt, 3 FirstSeqId.
    """
    rows = []
    row_count, pos = read_array_len(blob, 0)
    for _ in range(row_count):
        col_count, p = read_array_len(blob, pos)
        cp = p
        col_pos = []
        for _ in range(col_count):
            col_pos.append(cp)
            cp = skip_msgpack_value(blob, cp)
        sched_id  = read_int(blob, col_pos[0])
        start_dt  = struct.unpack('>q', blob[col_pos[1] + 1:col_pos[1] + 9])[0] if blob[col_pos[1]] == 0xd3 else 0
        first_seq = read_int(blob, col_pos[3])
        rows.append((first_seq, start_dt, sched_id, col_pos[2]))
        pos = cp

    canonical = {}
    for first_seq, start_dt, sched_id, _ in rows:
        cur = canonical.get(first_seq)
        if cur is None or (start_dt, sched_id) < cur:
            canonical[first_seq] = (start_dt, sched_id)

    zeroed = 0
    for first_seq, start_dt, sched_id, end_pos in rows:
        if (start_dt, sched_id) == canonical[first_seq]:
            continue
        if blob[end_pos] != 0xd3:
            continue
        struct.pack_into('>q', blob, end_pos + 1, 0)
        zeroed += 1
    return zeroed


def patch_wolf_chapter_battle_point(blob):
    """Chapter 314 ("Variation: Blazing Blossom") side-quests reference
    a BattlePointIndex that doesn't exist in the corresponding battle
    field locale asset, NRE'ing the client's
    TurnBattlePrefabAssetLoadApi.CreateFieldAssetLoadPlan during battle
    setup. The locale (bt_field_locale_eq000004_02) only has BattlePoints
    1-5, but BG 1880-1889 row col 6 says 8 — and the only BattlePoint
    whose _battleFieldPrefabIndex is 8 has _battlePointIndex 5, so 5 is
    the intended value (single-column-slip pattern).
    Idempotent: only rewrites col 6 when it's still 8.
    Columns: 0 BattleGroupId, 1 WaveNumber, 2 BattleId, 3 WaveStartActAssetId,
             4 WaveEndActAssetId, 5 BattleCameraControllerAssetId,
             6 BattlePointIndex, 7 BattleStartCameraType.
    """
    row_count, pos = read_array_len(blob, 0)
    patched = 0
    for _ in range(row_count):
        col_count, p = read_array_len(blob, pos)
        cp = p
        col_pos = []
        for _ in range(col_count):
            col_pos.append(cp)
            cp = skip_msgpack_value(blob, cp)
        bg_id = read_int(blob, col_pos[0])
        if 1880 <= bg_id <= 1889 and col_count > 6 and blob[col_pos[6]] == 0x08:
            # BattlePointIndex is a positive fixint (single byte 0x00-0x7F);
            # 8 → 5 is just a byte swap, no length recompute.
            blob[col_pos[6]] = 0x05
            patched += 1
        pos = cp
    return patched


def _subsumes(broad, narrow, family):
    """True if every entity/quest matching narrow targets also matches broad.

    broad/narrow are sorted tuples of (TargetType, TargetValue).
    Used by patch_campaign_dedup to drop redundant narrower-scope banners when
    a broader-scope row with the same effect+value already covers them.
    """
    broad_types = {t for t, _ in broad}
    if family == 'quest':
        if 1 in broad_types:                          # WHOLE_QUEST subsumes everything
            return True
        broad_qts = {v for t, v in broad if t == 2}   # QUEST_TYPE values present in broad
        if not broad_qts:
            return False
        for nt, nv in narrow:
            if nt == 1:                               # WHOLE only subsumes itself
                return False
            if nt == 2:                               # narrow QUEST_TYPE=V → broad must list V too
                if nv not in broad_qts: return False
            elif nt in (3, 6, 7):                     # event-side narrow → broad must cover EVENT
                if 2 not in broad_qts: return False
            elif nt in (4, 5):                        # main-side narrow → broad must cover MAIN
                if 1 not in broad_qts: return False
            else:
                return False
        return True
    if family == 'enhance':
        # *_ALL covers its sibling per-id / per-category target types.
        all_map = {1: {11, 12, 13}, 2: {21, 22, 23}, 3: {31, 32}}
        for nt, _ in narrow:
            covered_by_all = any(at in broad_types and nt in children
                                 for at, children in all_map.items())
            if not covered_by_all and nt not in broad_types:
                return False
        return True
    return False


def patch_campaign_dedup(camp_blob, target_rows, effect_rows, cfg, now_ms):
    """Dedup-extend a campaign table: pick one baseline-value rerun per
    (effect, target) tuple, drop entity-specific rows entirely, then drop
    any pick subsumed by another pick with the same effect+value+payload.
    """
    rows = msgpack.unpackb(bytes(camp_blob), raw=True)

    targets_by_group = {}
    for tg in target_rows:
        targets_by_group.setdefault(tg[0], []).append((tg[2], tg[3]))

    effects_by_id = None
    if effect_rows is not None:
        effects_by_id = {e[0]: (e[1], e[2], e[3]) for e in effect_rows}

    groups = {}      # key -> [(eff_val, camp_id, row_idx), ...] for reruns
    permanent = {}   # key -> (eff_val, target_tuple) for already-active-permanent rows
    for idx, row in enumerate(rows):
        if row[cfg['user_status_col']] != 1:
            continue
        targets = targets_by_group.get(row[cfg['tg_col']], [])
        if not targets or any(t[0] in cfg['entity_id_targets'] for t in targets):
            continue
        if effects_by_id is not None:
            eff = effects_by_id.get(row[cfg['eff_group_col']])
            if eff is None:
                continue
            eff_type, eff_val, item_group_id = eff
            extra = (item_group_id,)
        else:
            eff_type = row[cfg['eff_type_col']]
            eff_val  = row[cfg['eff_val_col']]
            extra = ()
        target_tuple = tuple(sorted(targets))
        key = (eff_type, target_tuple) + extra
        start_dt = row[cfg['start_col']]
        end_dt   = row[cfg['end_col']]
        if start_dt <= now_ms and end_dt >= MAX_PATCH_MS:
            permanent[key] = (eff_val, target_tuple)
        elif end_dt < MAX_PATCH_MS:
            groups.setdefault(key, []).append((eff_val, row[cfg['id_col']], idx))

    # Pass A: per key, pick the smallest-value rerun — but skip keys already
    # delivered by a permanent row (re-run idempotence).
    baseline = {}   # key -> (pick_value, pick_idx, target_tuple)
    for k, candidates in groups.items():
        if k in permanent:
            continue
        val, _, idx = min(candidates)
        baseline[k] = (val, idx, k[1])

    # Pass B: drop any pick subsumed by another permanent row OR baseline pick
    # in the same (effect, value [, item_group]) bucket.
    family = cfg['family']
    all_effective = list(permanent.items()) + [(k, (v, t)) for k, (v, _, t) in baseline.items()]
    picks = set()
    for k, (val, idx, targets) in baseline.items():
        bucket = (k[0], val) + k[2:]
        subsumed = False
        for k2, (val2, targets2) in all_effective:
            if k2 == k:
                continue
            if (k2[0], val2) + k2[2:] != bucket:
                continue
            if targets2 != targets and _subsumes(targets2, targets, family):
                subsumed = True
                break
        if not subsumed:
            picks.add(idx)

    row_count, pos = read_array_len(camp_blob, 0)
    end_col = cfg['end_col']
    bumped = 0
    for idx in range(row_count):
        col_count, p = read_array_len(camp_blob, pos)
        for ci in range(col_count):
            if ci == end_col and idx in picks and camp_blob[p] == 0xd3:
                struct.pack_into('>q', camp_blob, p + 1, TARGET_END_MS)
                bumped += 1
            p = skip_msgpack_value(camp_blob, p)
        pos = p
    return bumped


# --- Per-table apply helper ---

def apply_to_table(toc, data_blob, new_blobs, name, mutator):
    """Decompress name's blob (preferring new_blobs if already touched), run
    mutator(bytearray) → result, repack back into new_blobs. Returns mutator's
    return value or None if the table is absent.
    """
    if name not in toc:
        return None
    if name in new_blobs:
        src = bytes(new_blobs[name])
    else:
        offset, length = toc[name]
        src = data_blob[offset:offset + length]
    unpacked = msgpack.unpackb(src, raw=True)
    compressed = isinstance(unpacked, msgpack.ExtType) and unpacked.code == 99
    if compressed:
        unc_len, lz4_data = read_lz4_ext_header(unpacked.data)
        table = bytearray(lz4.block.decompress(lz4_data, uncompressed_size=unc_len))
    else:
        table = bytearray(src)
    result = mutator(table)
    if result is None and not compressed:
        # add_table_rows returns None to skip; preserve original bytes.
        return None
    if isinstance(result, (bytes, bytearray)):
        # Caller returned a fresh blob (add_table_rows); use it directly.
        new_blobs[name] = build_lz4_ext_blob(bytes(result)) if compressed else bytes(result)
        return True
    new_blobs[name] = build_lz4_ext_blob(bytes(table)) if compressed else bytes(table)
    return result


# --- Main ---

def main():
    parser = argparse.ArgumentParser(
        description="Patch master data timestamps to extend content to 2030.")
    key_group = parser.add_mutually_exclusive_group()
    key_group.add_argument("--key", default=DEFAULT_KEY, help="AES key as hex (default: built-in)")
    key_group.add_argument("--key-file", help="Raw key file (16 or 32 bytes)")
    iv_group = parser.add_mutually_exclusive_group()
    iv_group.add_argument("--iv", default=DEFAULT_IV, help="AES IV as hex (default: built-in)")
    iv_group.add_argument("--iv-file", help="Raw IV file (16 bytes)")
    parser.add_argument("--input", default=DEFAULT_INPUT, help=f"Input .bin.e (default: {DEFAULT_INPUT})")
    parser.add_argument("--output", help="Output .bin.e (default: overwrite input)")
    parser.add_argument("--dry-run", action="store_true", help="Decrypt + patch but don't write")
    args = parser.parse_args()

    key = open(args.key_file, "rb").read() if args.key_file else bytes.fromhex(args.key)
    if len(key) not in (16, 32):
        sys.exit(f"ERROR: AES key must be 16 or 32 bytes, got {len(key)}")
    iv = open(args.iv_file, "rb").read() if args.iv_file else bytes.fromhex(args.iv)
    if len(iv) != 16:
        sys.exit(f"ERROR: AES IV must be 16 bytes, got {len(iv)}")
    aes_bits = len(key) * 8
    output_path = args.output or args.input

    print(f"Reading {args.input}...")
    with open(args.input, "rb") as f:
        encrypted = f.read()
    print(f"  Encrypted size: {len(encrypted)} bytes")

    print(f"Decrypting (AES-{aes_bits}-CBC)...")
    try:
        decrypted = unpad(AES.new(key, AES.MODE_CBC, iv).decrypt(encrypted), AES.block_size)
    except ValueError as e:
        sys.exit(f"ERROR: Decryption failed: {e}\n  Check that the key and IV are correct.")
    print(f"  Decrypted size: {len(decrypted)} bytes")

    print("Parsing MasterMemory header...")
    try:
        toc = msgpack.unpackb(decrypted, raw=False, strict_map_key=False)
        data_blob = b""
    except msgpack.ExtraData as e:
        toc = e.unpacked
        data_blob = e.extra
    if not isinstance(toc, dict):
        sys.exit(f"ERROR: Expected dict header, got {type(toc).__name__}")
    print(f"  {len(toc)} tables, data blob: {len(data_blob)} bytes")

    print(f"\nPatching EndDatetime fields (target: {TARGET_END_DT.isoformat()})...")
    new_blobs = {}
    stats = {}
    total_patched = 0
    for tname, columns in PATCH_COLUMNS.items():
        if tname in SKIP_TABLES:
            continue
        col_indices = {idx for idx, _ in columns}
        row_filter = TABLE_PATCH_FILTERS.get(tname)
        result = apply_to_table(toc, data_blob, new_blobs, tname,
                                lambda b: patch_table_blob(b, col_indices, row_filter))
        if result is None:
            continue
        count, skip_count = result
        if count > 0:
            stats[tname] = (count, skip_count)
            total_patched += count
        else:
            # No patchable rows — undo the (no-op) repack so file output stays minimal.
            del new_blobs[tname]

    print(f"\n  Patched {total_patched} values across {len(stats)} tables:")
    for tname in sorted(stats):
        count, skip_count = stats[tname]
        cols = ", ".join(name for _, name in PATCH_COLUMNS[tname])
        suffix = f" (skipped {skip_count} rows by filter)" if skip_count else ""
        print(f"    {tname}: {count} values ({cols}){suffix}")

    print("\nPulling future StartDatetime fields back (parked content)...")
    start_total = 0
    for tname, columns in PULL_START_COLUMNS.items():
        col_indices = {idx for idx, _ in columns}
        result = apply_to_table(toc, data_blob, new_blobs, tname,
                                lambda b, ci=col_indices: pull_start_blob(b, ci))
        if result:
            print(f"    {tname}: pulled {result} start date(s) back")
            start_total += result
    tgt = datetime.fromtimestamp(START_PULLBACK_TARGET_MS / 1000, tz=timezone.utc).date()
    print(f"  Pulled {start_total} future start(s) back to {tgt}")

    emptied = []
    for tname in sorted(EMPTY_TABLES):
        if tname in toc:
            new_blobs[tname] = msgpack.packb([], use_bin_type=True)
            emptied.append(tname)
    if emptied:
        print(f"\n  Emptied tables: {', '.join(emptied)}")
    if SKIP_TABLES:
        print(f"\n  Skipped tables: {', '.join(sorted(SKIP_TABLES))}")

    added = []
    for tname, rows in TABLE_ROW_ADDITIONS.items():
        result = apply_to_table(toc, data_blob, new_blobs, tname,
                                lambda b, _rows=rows: add_table_rows(b, _rows))
        if result is None:
            print(f"  {tname}: not in TOC or row(s) already present, skipping")
        else:
            added.append(f"{tname} (+{len(rows)} row)")
    if added:
        print(f"\n  Added rows: {', '.join(added)}")

    zeroed = apply_to_table(toc, data_blob, new_blobs, 'm_gimmick_sequence_schedule',
                            patch_gimmick_sequence_schedules)
    if zeroed is None:
        print("  WARNING: m_gimmick_sequence_schedule not in TOC, skipping")
    else:
        print(f"\n  Gimmick sequence schedules: {zeroed} duplicate rows expired (1 active per FirstGimmickSequenceId)")

    now_ms = int(datetime.now(tz=timezone.utc).timestamp() * 1000)
    for family, cfg in CAMPAIGN_CFG.items():
        camp_table = f'm_{family}_campaign'
        if camp_table not in toc or cfg['target_table'] not in toc:
            print(f"  WARNING: {camp_table} / {cfg['target_table']} not in TOC, skipping dedup")
            continue
        target_rows  = _load_decoded_table(toc, data_blob, cfg['target_table'])
        effect_rows  = _load_decoded_table(toc, data_blob, cfg['effect_table']) if cfg['effect_table'] else None
        if cfg['effect_table'] and effect_rows is None:
            print(f"  WARNING: {cfg['effect_table']} not in TOC, skipping {family} dedup")
            continue
        bumped = apply_to_table(toc, data_blob, new_blobs, camp_table,
                                lambda b, t=target_rows, e=effect_rows: patch_campaign_dedup(b, t, e, cfg, now_ms))
        print(f"\n  {camp_table}: dedup-extended {bumped} rows (baseline-value per unique (effect, target))")

    written = apply_to_table(toc, data_blob, new_blobs, 'm_event_quest_labyrinth_season',
                             patch_labyrinth_seasons)
    if written is None:
        print("  WARNING: m_event_quest_labyrinth_season not in TOC, skipping")
    else:
        print(f"\n  Labyrinth seasons: {written} rows windowed (1 active per chapter)")

    bp_fixed = apply_to_table(toc, data_blob, new_blobs, 'm_battle_group',
                              patch_wolf_chapter_battle_point)
    if bp_fixed is None:
        print("  WARNING: m_battle_group not in TOC, skipping wolf-chapter fix")
    else:
        print(f"\n  Wolf chapter (314) BattlePointIndex: {bp_fixed} rows corrected 8->5 (BG 1880-1889)")

    if args.dry_run:
        print("\n[DRY RUN] Skipping rebuild and encryption.")
        return

    print("\nRebuilding MasterMemory binary...")
    sorted_tables = sorted(toc.items(), key=lambda kv: kv[1][0])
    new_toc = {}
    blob_parts = []
    current_offset = 0
    for tname, (orig_offset, orig_length) in sorted_tables:
        part = new_blobs[tname] if tname in new_blobs else data_blob[orig_offset:orig_offset + orig_length]
        new_toc[tname] = (current_offset, len(part))
        blob_parts.append(part)
        current_offset += len(part)
    new_data_blob = b''.join(blob_parts)
    new_header = msgpack.packb(new_toc, use_bin_type=True)
    new_decrypted = new_header + new_data_blob
    print(f"  Header: {len(new_header)} bytes, blob: {len(new_data_blob)} bytes")
    print(f"  Total: {len(new_decrypted)} bytes (original: {len(decrypted)})")

    print(f"Re-encrypting (AES-{aes_bits}-CBC)...")
    re_encrypted = AES.new(key, AES.MODE_CBC, iv).encrypt(pad(new_decrypted, AES.block_size))
    print(f"  Re-encrypted size: {len(re_encrypted)} bytes")

    print(f"Writing {output_path}...")
    with open(output_path, "wb") as f:
        f.write(re_encrypted)
    print(f"  Done! Patched binary written to {output_path}")


def _load_decoded_table(toc, data_blob, name):
    """Decompress + msgpack-decode a table; returns the list of rows (each a list of column values)."""
    if name not in toc:
        return None
    offset, length = toc[name]
    src = data_blob[offset:offset + length]
    unpacked = msgpack.unpackb(src, raw=True)
    if isinstance(unpacked, msgpack.ExtType) and unpacked.code == 99:
        unc_len, lz4_data = read_lz4_ext_header(unpacked.data)
        raw = lz4.block.decompress(lz4_data, uncompressed_size=unc_len)
        return msgpack.unpackb(raw, raw=True)
    return unpacked


if __name__ == "__main__":
    main()
