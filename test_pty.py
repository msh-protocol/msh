import sys
import json

is_tty = sys.stdout.isatty()
print(json.dumps({"is_tty": is_tty}))
