#!/usr/bin/env python3
"""Parallel download of NieR Reincarnation master data from archive.org"""

import urllib.request
import os
import re
from concurrent.futures import ThreadPoolExecutor, as_completed
from html.parser import HTMLParser
import sys

class LinkExtractor(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        if tag == 'a':
            for attr in attrs:
                if attr[0] == 'href':
                    self.links.append(attr[1])

BASE_URL = "https://archive.org/download/nierreincarnation/Global/master_data/"
OUTPUT_DIR = "master_data"
MAX_WORKERS = 10  # Parallel downloads

os.makedirs(OUTPUT_DIR, exist_ok=True)

def download_file(filename):
    """Download a single file"""
    url = BASE_URL + filename
    filepath = os.path.join(OUTPUT_DIR, filename)

    # Skip if already exists and has content
    if os.path.exists(filepath) and os.path.getsize(filepath) > 100:
        return (filename, "skipped", os.path.getsize(filepath))

    try:
        urllib.request.urlretrieve(url, filepath)
        size = os.path.getsize(filepath)
        return (filename, "ok", size)
    except Exception as e:
        return (filename, f"error: {e}", 0)

def main():
    print("Fetching file list from archive.org...")
    try:
        response = urllib.request.urlopen(BASE_URL, timeout=30)
        html = response.read().decode('utf-8')

        parser = LinkExtractor()
        parser.feed(html)

        # Filter for .json files
        json_files = [link for link in parser.links if link.endswith('.json') and link != '../']

        print(f"Found {len(json_files)} JSON files")
        print(f"Downloading with {MAX_WORKERS} parallel workers...\n")

        # Parallel download
        completed = 0
        failed = 0
        skipped = 0
        total_size = 0

        with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
            futures = {executor.submit(download_file, f): f for f in json_files}

            for future in as_completed(futures):
                filename, status, size = future.result()
                completed += 1

                if status == "ok":
                    total_size += size
                    print(f"[{completed}/{len(json_files)}] ✓ {filename} ({size:,} bytes)")
                elif status == "skipped":
                    skipped += 1
                    total_size += size
                    print(f"[{completed}/{len(json_files)}] ○ {filename} (already exists, {size:,} bytes)")
                else:
                    failed += 1
                    print(f"[{completed}/{len(json_files)}] ✗ {filename} ({status})")

        print(f"\n{'='*50}")
        print(f"Download complete!")
        print(f"  Total files: {len(json_files)}")
        print(f"  Downloaded:  {completed - skipped - failed}")
        print(f"  Skipped:     {skipped}")
        print(f"  Failed:      {failed}")
        print(f"  Total size:  {total_size / 1024 / 1024:.1f} MB")
        print(f"{'='*50}")

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
