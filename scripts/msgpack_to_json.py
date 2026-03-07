#!/usr/bin/env python3
"""Convert all msgpack files in a directory to JSON."""

import os
import sys
import json
import msgpack
from pathlib import Path


def convert_file(input_path, output_path):
    """Convert single msgpack file to JSON."""
    try:
        with open(input_path, 'rb') as f:
            data = msgpack.load(f)

        with open(output_path, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=2, ensure_ascii=False, default=str)

        return True
    except Exception as e:
        print(f"Error converting {input_path}: {e}")
        return False


def main():
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <input_dir> <output_dir>")
        sys.exit(1)

    input_dir = Path(sys.argv[1])
    output_dir = Path(sys.argv[2])

    if not input_dir.exists():
        print(f"Input directory not found: {input_dir}")
        sys.exit(1)

    output_dir.mkdir(parents=True, exist_ok=True)

    msgpack_files = list(input_dir.glob("*.msgpack"))
    print(f"Found {len(msgpack_files)} msgpack files")

    converted = 0
    failed = 0

    for i, msgpack_file in enumerate(msgpack_files, 1):
        json_file = output_dir / (msgpack_file.stem + ".json")

        if convert_file(msgpack_file, json_file):
            converted += 1
            print(f"[{i}/{len(msgpack_files)}] {msgpack_file.name} -> {json_file.name}")
        else:
            failed += 1

    print(f"\nDone: {converted} converted, {failed} failed")
    print(f"JSON files in: {output_dir}")


if __name__ == '__main__':
    main()
