#!/usr/bin/env python3
"""CLI entry point for injectived binary."""

import sys
from . import run_binary


def main():
    """Main entry point for the injectived command."""
    args = sys.argv[1:] if len(sys.argv) > 1 else []
    
    try:
        run_binary(args)
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error running injectived: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
