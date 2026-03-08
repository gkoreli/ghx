NAME="Code search"
DESCRIPTION="Search for a multi-word query in a repo. Tests two queries: one where both tools find results (AND-friendly), one where gh search code fails due to exact-phrase wrapping."
REPO="plausible/analytics"
QUERY="bar width repo:$REPO"
QUERY2="ghx gkoreli"

cmd_a() {
  echo "--- Query 1: '$QUERY' ---"
  ghx search "$QUERY"
  echo ""
  echo "--- Query 2: '$QUERY2' (gh search code wraps in quotes = exact phrase, finds nothing) ---"
  ghx search "$QUERY2"
}
cmd_a_label="ghx search"
cmd_a_calls=2

cmd_b() {
  echo "--- Query 1: '$QUERY' ---"
  gh search code "$QUERY" 2>&1
  echo ""
  echo "--- Query 2: '$QUERY2' (gh search code wraps in quotes = exact phrase, finds nothing) ---"
  gh search code "$QUERY2" 2>&1
}
cmd_b_label="gh search code"
cmd_b_calls=2
