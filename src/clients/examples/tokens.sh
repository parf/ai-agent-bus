#!/bin/bash
# Prints the token run.sh minted for an agent: tokens.sh '#stats-php@localhost'
cat "$EXAMPLES_TOKENS/$(printf '%s' "$1" | tr '#/@' '___')"
