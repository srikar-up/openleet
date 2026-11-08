#!/usr/bin/env python3
"""Convert PNG captures under readme/screenshots to WebP.

The gallery is shipped as WebP at quality 88. VHS still writes PNG, so run this
after a capture session:

    python scripts/convert-screenshots.py --delete

Requires Pillow (``python -m pip install pillow``).
"""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default="readme/screenshots", help="capture directory")
    parser.add_argument("--quality", type=int, default=88, help="WebP quality (default: 88)")
    parser.add_argument("--delete", action="store_true", help="remove the source PNG after conversion")
    args = parser.parse_args()

    root = Path(args.root)
    sources = sorted(root.rglob("*.png"))
    if not sources:
        print(f"no PNG files under {root}")
        return 0

    before = after = 0
    for source in sources:
        target = source.with_suffix(".webp")
        with Image.open(source) as image:
            size = image.size
            before += source.stat().st_size
            image.save(target, "WEBP", quality=args.quality, method=6)
        with Image.open(target) as check:
            if check.size != size:
                raise SystemExit(f"size mismatch: {source} -> {target}")
        after += target.stat().st_size
        if args.delete:
            source.unlink()
        print(f"{source} -> {target}")

    print(f"converted {len(sources)} files: {before / 1048576:.2f} MB -> {after / 1048576:.2f} MB")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
