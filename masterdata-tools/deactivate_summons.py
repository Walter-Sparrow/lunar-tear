#!/usr/bin/env python3
"""Deactivate all gacha summon banners except a keep-list.

Sets m_mom_banner.EndDatetime (col 7) to a past date for every banner whose
MomBannerId is NOT in --keep, so only the kept banners stay available. Reuses
patch_masterdata.py's machinery.

Requires: pip install pycryptodome msgpack lz4
"""

import argparse
import struct

from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad

import patch_masterdata as pm

# m_mom_banner columns: MomBannerId(0) ... StartDatetime(6), EndDatetime(7)
ID_COL = 0
START_COL = 6
END_COL = 7
PAST_MS = pm.MIN_PATCH_MS          # 2020-01-01 — closed (past) end for deactivated banners
ACTIVE_START_MS = pm.MIN_PATCH_MS  # 2020-01-01 — open start for kept banners
ACTIVE_END_MS = pm.TARGET_END_MS   # 2030-12-31 — open end for kept banners


def deactivate_except(blob, keep_ids):
    """Keep: open window so it's active. Others: push End into the past."""
    row_count, pos = pm.read_array_len(blob, 0)
    deactivated = activated = 0
    for _ in range(row_count):
        col_count, p = pm.read_array_len(blob, pos)
        banner_id = pm.read_int(blob, p)  # col 0
        keep = banner_id in keep_ids
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
    ap = argparse.ArgumentParser(description="Deactivate all summon banners except --keep.")
    ap.add_argument("--input", default=pm.DEFAULT_INPUT)
    ap.add_argument("--output", help="default: overwrite input")
    ap.add_argument("--keep", type=int, nargs="*", default=[],
                    help="MomBannerId(s) to keep active (default: none = deactivate all)")
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

    if "m_mom_banner" not in toc:
        raise SystemExit("m_mom_banner not in master data")

    new_blobs = {}
    result = pm.apply_to_table(toc, data_blob, new_blobs, "m_mom_banner",
                               lambda b: deactivate_except(b, keep))
    deact, act = result if result else (0, 0)
    print(f"summons: deactivated {deact}, kept active {act} (keep: {sorted(keep)})")

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
