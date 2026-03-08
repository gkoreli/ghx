NAME="Code search"
DESCRIPTION="Search for a multi-word query in a repo"
REPO="plausible/analytics"
QUERY="bar width repo:$REPO"

cmd_a() { ghx search "$QUERY"; }
cmd_a_label="ghx search"
cmd_a_calls=1

cmd_b() { gh search code "$QUERY" 2>&1; }
cmd_b_label="gh search code"
cmd_b_calls=1
