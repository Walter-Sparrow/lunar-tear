#!/usr/bin/env python3
"""Deactivate all event-quest chapters except a keep-list.

Sets m_event_quest_chapter.EndDatetime to a past date for every chapter whose
EventQuestChapterId is NOT in --keep, so only the kept chapter(s) stay active.
Reuses patch_masterdata.py's decrypt / msgpack-walker / encrypt machinery.

Requires: pip install pycryptodome msgpack lz4
"""

import argparse
import struct

from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad

import patch_masterdata as pm

# m_event_quest_chapter columns: EventQuestChapterId(0) ... StartDatetime(8), EndDatetime(9)
ID_COL = 0
START_COL = 8
END_COL = 9
PAST_MS = pm.MIN_PATCH_MS      # 2020-01-01 — a closed (past) end for deactivated chapters
ACTIVE_START_MS = pm.MIN_PATCH_MS   # 2020-01-01 — open start for kept chapters
ACTIVE_END_MS = pm.TARGET_END_MS    # 2030-12-31 — open end for kept chapters


def deactivate_except(blob, keep_ids):
    """For kept chapters: open the window (Start->2020, End->2030-12-31) so they
    are active. For all others: push End into the past so they are inactive.
    Returns (deactivated, activated)."""
    row_count, pos = pm.read_array_len(blob, 0)
    deactivated = activated = 0
    for _ in range(row_count):
        col_count, p = pm.read_array_len(blob, pos)
        chapter_id = pm.read_int(blob, p)  # col 0
        keep = chapter_id in keep_ids
        cp = p
        for ci in range(col_count):
            if blob[cp] == 0xd3:
                if keep and ci == START_COL:
                    struct.pack_into('>q', blob, cp + 1, ACTIVE_START_MS)
                elif keep and ci == END_COL:
                    struct.pack_into('>q', blob, cp + 1, ACTIVE_END_MS)
                    activated += 1
                elif not keep and ci == END_COL:
                    struct.pack_into('>q', blob, cp + 1, PAST_MS)
                    deactivated += 1
            cp = pm.skip_msgpack_value(blob, cp)
        pos = cp
    return deactivated, activated


def main():
    ap = argparse.ArgumentParser(description="Deactivate all event-quest chapters except --keep.")
    ap.add_argument("--input", default=pm.DEFAULT_INPUT)
    ap.add_argument("--output", help="default: overwrite input")
    ap.add_argument("--keep", type=int, nargs="+", required=True,
                    help="EventQuestChapterId(s) to keep active")
    args = ap.parse_args()

    key = bytes.fromhex(pm.DEFAULT_KEY)
    iv = bytes.fromhex(pm.DEFAULT_IV)
    out = args.output or args.input
    keep = set(args.keep)

    encrypted = open(args.input, "rb").read()
    decrypted = unpad(AES.new(key, AES.MODE_CBC, iv).decrypt(encrypted), AES.block_size)

    try:
        toc = pm.msgpack.unpackb(decrypted, raw=False, strict_map_key=False)
        data_blob = b""
    except pm.msgpack.ExtraData as e:
        toc = e.unpacked
        data_blob = e.extra

    if "m_event_quest_chapter" not in toc:
        raise SystemExit("m_event_quest_chapter not in master data")

    new_blobs = {}
    result = pm.apply_to_table(toc, data_blob, new_blobs, "m_event_quest_chapter",
                               lambda b: deactivate_except(b, keep))
    deact, act = result if result else (0, 0)
    print(f"event quests: deactivated {deact}, activated {act} (kept: {sorted(keep)})")

    # Rebuild (same as patch_masterdata.main()).
    sorted_tables = sorted(toc.items(), key=lambda kv: kv[1][0])
    new_toc = {}
    parts = []
    offset = 0
    for tname, (o, l) in sorted_tables:
        part = new_blobs[tname] if tname in new_blobs else data_blob[o:o + l]
        new_toc[tname] = (offset, len(part))
        parts.append(part)
        offset += len(part)
    new_decrypted = pm.msgpack.packb(new_toc, use_bin_type=True) + b"".join(parts)
    re_encrypted = AES.new(key, AES.MODE_CBC, iv).encrypt(pad(new_decrypted, AES.block_size))
    open(out, "wb").write(re_encrypted)
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
