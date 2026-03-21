NAME="Code search: ghx vs gh"
DESCRIPTION="Compare ghx search (AND + text_matches + token protection) vs gh search code (exact phrase, no context).
ghx sends unquoted AND queries, shows matching lines, truncates to 200 chars, reports count on stderr.
gh search code silently wraps in quotes (exact phrase), shows only paths, no match context."

commands=(
  'ghx search "bar width repo:plausible/analytics"'
  'gh search code "bar width repo:plausible/analytics"'
  'ghx search "ghx gkoreli"'
  'gh search code "ghx gkoreli"'
  'ghx search "addClass repo:jquery/jquery"'
  'gh search code "addClass repo:jquery/jquery"'
  'ghx search "jQuery.fn.extend filename:jquery.min.js"'
  'gh search code "jQuery.fn.extend filename:jquery.min.js"'
  'ghx search "useState language:typescript"'
  'gh search code "useState language:typescript"'
)
