NAME="Code search"
DESCRIPTION="Search for a multi-word query. Tests AND matching (ghx) vs exact-phrase matching (gh search code wraps in quotes)."

commands=(
  'ghx search "bar width repo:plausible/analytics"'
  'gh search code "bar width repo:plausible/analytics"'
  'ghx search "ghx gkoreli"'
  'gh search code "ghx gkoreli"'
)
