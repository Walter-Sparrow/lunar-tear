#!/usr/bin/env python3
"""
Extract master data tables from full binary dump.
Usage: python extract_master_data.py <master_data_full.bin> <output_dir>
"""

import sys
import os
import msgpack


def read_msgpack_element_size(data, offset):
    """Calculate size of msgpack element at given offset."""
    if offset >= len(data):
        return 0

    byte = data[offset]

    # Positive fixint: 0x00-0x7f
    if byte <= 0x7f:
        return 1

    # Fixmap: 0x80-0x8f
    if 0x80 <= byte <= 0x8f:
        count = byte & 0x0f
        size = 1
        pos = offset + 1
        for _ in range(count):
            key_size = read_msgpack_element_size(data, pos)
            if key_size == 0:
                return 0
            pos += key_size
            size += key_size
            val_size = read_msgpack_element_size(data, pos)
            if val_size == 0:
                return 0
            pos += val_size
            size += val_size
        return size

    # Fixarray: 0x90-0x9f
    if 0x90 <= byte <= 0x9f:
        count = byte & 0x0f
        size = 1
        pos = offset + 1
        for _ in range(count):
            el_size = read_msgpack_element_size(data, pos)
            if el_size == 0:
                return 0
            pos += el_size
            size += el_size
        return size

    # Fixstr: 0xa0-0xbf
    if 0xa0 <= byte <= 0xbf:
        return 1 + (byte & 0x1f)

    # Null: 0xc0
    if byte == 0xc0:
        return 1

    # False/True: 0xc2-0xc3
    if 0xc2 <= byte <= 0xc3:
        return 1

    # Bin8: 0xc4
    if byte == 0xc4:
        return 2 + data[offset + 1]

    # Bin16: 0xc5
    if byte == 0xc5:
        return 3 + int.from_bytes(data[offset + 1:offset + 3], 'big')

    # Bin32: 0xc6
    if byte == 0xc6:
        return 5 + int.from_bytes(data[offset + 1:offset + 5], 'big')

    # Float32: 0xca
    if byte == 0xca:
        return 5

    # Float64: 0xcb
    if byte == 0xcb:
        return 9

    # Uint8: 0xcc
    if byte == 0xcc:
        return 2

    # Uint16: 0xcd
    if byte == 0xcd:
        return 3

    # Uint32: 0xce
    if byte == 0xce:
        return 5

    # Uint64: 0xcf
    if byte == 0xcf:
        return 9

    # Int8: 0xd0
    if byte == 0xd0:
        return 2

    # Int16: 0xd1
    if byte == 0xd1:
        return 3

    # Int32: 0xd2
    if byte == 0xd2:
        return 5

    # Int64: 0xd3
    if byte == 0xd3:
        return 9

    # Str8: 0xd9
    if byte == 0xd9:
        return 2 + data[offset + 1]

    # Str16: 0xda
    if byte == 0xda:
        return 3 + int.from_bytes(data[offset + 1:offset + 3], 'big')

    # Str32: 0xdb
    if byte == 0xdb:
        return 5 + int.from_bytes(data[offset + 1:offset + 5], 'big')

    # Array16: 0xdc
    if byte == 0xdc:
        count = int.from_bytes(data[offset + 1:offset + 3], 'big')
        size = 3
        pos = offset + 3
        for _ in range(count):
            el_size = read_msgpack_element_size(data, pos)
            if el_size == 0:
                return 0
            pos += el_size
            size += el_size
        return size

    # Array32: 0xdd
    if byte == 0xdd:
        count = int.from_bytes(data[offset + 1:offset + 5], 'big')
        size = 5
        pos = offset + 5
        for _ in range(count):
            el_size = read_msgpack_element_size(data, pos)
            if el_size == 0:
                return 0
            pos += el_size
            size += el_size
        return size

    # Map16: 0xde
    if byte == 0xde:
        count = int.from_bytes(data[offset + 1:offset + 3], 'big')
        size = 3
        pos = offset + 3
        for _ in range(count):
            key_size = read_msgpack_element_size(data, pos)
            if key_size == 0:
                return 0
            pos += key_size
            size += key_size
            val_size = read_msgpack_element_size(data, pos)
            if val_size == 0:
                return 0
            pos += val_size
            size += val_size
        return size

    # Map32: 0xdf
    if byte == 0xdf:
        count = int.from_bytes(data[offset + 1:offset + 5], 'big')
        size = 5
        pos = offset + 5
        for _ in range(count):
            key_size = read_msgpack_element_size(data, pos)
            if key_size == 0:
                return 0
            pos += key_size
            size += key_size
            val_size = read_msgpack_element_size(data, pos)
            if val_size == 0:
                return 0
            pos += val_size
            size += val_size
        return size

    # Negative fixint: 0xe0-0xff
    if byte >= 0xe0:
        return 1

    # Unknown
    print(f"WARNING: Unknown msgpack byte 0x{byte:02x} at offset {offset}")
    return 1


def read_msgpack_string(data, offset):
    """Read msgpack string at given offset."""
    if offset >= len(data):
        return None

    byte = data[offset]

    # Fixstr: 0xa0-0xbf
    if 0xa0 <= byte <= 0xbf:
        length = byte & 0x1f
        return data[offset + 1:offset + 1 + length].decode('utf-8')

    # Str8: 0xd9
    if byte == 0xd9:
        length = data[offset + 1]
        return data[offset + 2:offset + 2 + length].decode('utf-8')

    # Str16: 0xda
    if byte == 0xda:
        length = int.from_bytes(data[offset + 1:offset + 3], 'big')
        return data[offset + 3:offset + 3 + length].decode('utf-8')

    # Str32: 0xdb
    if byte == 0xdb:
        length = int.from_bytes(data[offset + 1:offset + 5], 'big')
        return data[offset + 5:offset + 5 + length].decode('utf-8')

    return None


def extract_tables(input_file, output_dir):
    """Extract tables from master data file."""
    os.makedirs(output_dir, exist_ok=True)

    with open(input_file, 'rb') as f:
        data = f.read()

    print(f"Loaded {len(data)} bytes")

    # Parse map header
    if len(data) < 1:
        print("Empty file")
        return

    map_byte = data[0]
    table_count = 0
    offset = 1

    if 0x80 <= map_byte <= 0x8f:
        table_count = map_byte & 0x0f
    elif map_byte == 0xde:
        table_count = int.from_bytes(data[1:3], 'big')
        offset = 3
    elif map_byte == 0xdf:
        table_count = int.from_bytes(data[1:5], 'big')
        offset = 5
    else:
        print(f"Unknown map byte: 0x{map_byte:02x}")
        return

    print(f"Found {table_count} tables")

    extracted = 0
    empty = 0

    for i in range(table_count):
        try:
            # Read table name (key)
            table_name = read_msgpack_string(data, offset)
            if table_name is None:
                print(f"Table {i}: Could not read name (byte: 0x{data[offset]:02x})")
                offset += 1
                continue

            key_size = read_msgpack_element_size(data, offset)
            offset += key_size

            # Read table data size
            value_size = read_msgpack_element_size(data, offset)

            if value_size == 0:
                print(f"WARNING: Table {i} '{table_name}' has size=0")
                empty += 1
                offset += 1
                continue

            # Extract table data
            table_data = data[offset:offset + value_size]

            # Save to file
            padded_index = str(i).zfill(3)
            output_path = os.path.join(output_dir, f"{padded_index}_{table_name}.msgpack")

            with open(output_path, 'wb') as f:
                f.write(table_data)

            print(f"Table {i}: '{table_name}' -> {output_path} ({value_size} bytes)")
            extracted += 1
            offset += value_size

        except Exception as e:
            print(f"Error extracting table {i}: {e}")
            break

    print(f"\nExtraction complete: {extracted} tables saved ({empty} empty skipped)")


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <master_data_full.bin> <output_dir>")
        sys.exit(1)

    input_file = sys.argv[1]
    output_dir = sys.argv[2]

    if not os.path.exists(input_file):
        print(f"Input file not found: {input_file}")
        sys.exit(1)

    extract_tables(input_file, output_dir)
