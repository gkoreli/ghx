# Code search

Search for a multi-word query in a repo. Tests two queries: one where both tools find results (AND-friendly), one where gh search code fails due to exact-phrase wrapping.

## Results

| Method | Time (ms) | Bytes | Tokens | Lines | API Calls |
|--------|-----------|-------|--------|-------|-----------|
| ghx search | 999ms |     1741 | 420 | 24 | 2 |
| gh search code | 585ms |     3971 | 995 | 39 | 2 |

## Method A: ghx search

```
--- Query 1: 'bar width repo:plausible/analytics' ---
plausible/analytics assets/js/dashboard/stats/bar.js:bar.js
plausible/analytics assets/js/dashboard/nav-menu/filters-bar.tsx:filters-bar.tsx
plausible/analytics assets/js/dashboard/nav-menu/top-bar.tsx:top-bar.tsx
plausible/analytics assets/js/dashboard/extra/funnel.js:funnel.js
plausible/analytics assets/js/dashboard/stats/reports/list.tsx:list.tsx
plausible/analytics lib/plausible_web/components/billing/billing.ex:billing.ex
plausible/analytics assets/css/app.css:app.css
plausible/analytics CHANGELOG.md:CHANGELOG.md
plausible/analytics lib/plausible_web/templates/settings/api_keys.html.heex:api_keys.html.heex
plausible/analytics lib/plausible_web/live/goal_settings/list.ex:list.ex
plausible/analytics lib/plausible_web/live/sites.ex:sites.ex
plausible/analytics lib/plausible_web/live/shields/ip_rules.ex:ip_rules.ex
plausible/analytics lib/plausible_web/components/generic.ex:generic.ex
plausible/analytics assets/js/dashboard/stats/sources/search-terms.tsx:search-terms.tsx
plausible/analytics lib/plausible_web/live/shields/page_rules.ex:page_rules.ex
plausible/analytics assets/js/dashboard/nav-menu/filters-bar.test.tsx:filters-bar.test.tsx
plausible/analytics test/plausible_web/components/billing/billing_test.exs:billing_test.exs
plausible/analytics tracker/test/fixtures/legacy-pageview-properties.html:legacy-pageview-properties.html
plausible/analytics tracker/test/fixtures/cookies-onetrust.html:cookies-onetrust.html
plausible/analytics tracker/test/fixtures/cookies-cookiebot.html:cookies-cookiebot.html

--- Query 2: 'ghx gkoreli' (gh search code wraps in quotes = exact phrase, finds nothing) ---
alphaleadership/npm-check docs/pending-db.json:pending-db.json
```

## Method B: gh search code

```
--- Query 1: 'bar width repo:plausible/analytics' ---
plausible/analytics:assets/js/dashboard/stats/bar.js: export default function Bar({
plausible/analytics:assets/js/dashboard/nav-menu/filters-bar.tsx: ) => (element === null ? null : element.getBoundingClientRect().width)
plausible/analytics:assets/js/dashboard/nav-menu/filters-bar.tsx: width: number
plausible/analytics:assets/js/dashboard/nav-menu/top-bar.tsx: import { FiltersBar } from './filters-bar'
plausible/analytics:assets/js/dashboard/nav-menu/top-bar.tsx: 'sticky fullwidth-shadow bg-gray-50 dark:bg-gray-950'
plausible/analytics:assets/js/dashboard/extra/funnel.js: c.fillRect(0, 0, shape.width, shape.height)
plausible/analytics:assets/js/dashboard/stats/reports/list.tsx: <Bar
plausible/analytics:assets/js/dashboard/stats/reports/list.tsx: maxWidthDeduction={undefined}
plausible/analytics:lib/plausible_web/components/billing/billing.ex: |> assign(:color_class, progress_bar_color_from_percentage(percentage, assigns.limit))
plausible/analytics:lib/plausible_web/components/billing/billing.ex: style={"width: #{@percentage}%"}
plausible/analytics:assets/css/app.css: width: 100vw; /* Prevents content from jumping when scrollbar is added/removed due to vertical overflow */
plausible/analytics:CHANGELOG.md: - Fix `width=manual` in embedded dashboards plausible/analytics#3910
plausible/analytics:CHANGELOG.md: - UI fixes for text not showing properly in bars across multiple lines. This hides the totals on <768px and only shows the uniques and % to accommodate the goals text too. Larger screens still truncate as usual.
plausible/analytics:lib/plausible_web/templates/settings/api_keys.html.heex: </.filter_bar>
plausible/analytics:lib/plausible_web/templates/settings/api_keys.html.heex: <.td truncate max_width="max-w-40">
plausible/analytics:lib/plausible_web/live/goal_settings/list.ex: </.filter_bar>
plausible/analytics:lib/plausible_web/live/goal_settings/list.ex: <.td max_width="max-w-52 sm:max-w-64" height="h-16">
plausible/analytics:lib/plausible_web/live/sites.ex: <.filter_bar filter_text={@filter_text} placeholder="Search Sites"></.filter_bar>
plausible/analytics:lib/plausible_web/live/sites.ex: stroke-width="1.5"
plausible/analytics:lib/plausible_web/live/shields/ip_rules.ex: </.filter_bar>
plausible/analytics:lib/plausible_web/live/shields/ip_rules.ex: <.td max_width="max-w-40">
plausible/analytics:lib/plausible_web/components/generic.ex: stroke-width="1.5"
plausible/analytics:lib/plausible_web/components/generic.ex: def filter_bar(assigns) do
plausible/analytics:assets/js/dashboard/stats/sources/search-terms.tsx: <Bar
plausible/analytics:assets/js/dashboard/stats/sources/search-terms.tsx: maxWidthDeduction="4rem"
plausible/analytics:lib/plausible_web/live/shields/page_rules.ex: </.filter_bar>
plausible/analytics:lib/plausible_web/live/shields/page_rules.ex: <.td max_width="max-w-40" truncate>
plausible/analytics:assets/js/dashboard/nav-menu/filters-bar.test.tsx: import { FiltersBar, handleVisibility } from './filters-bar'
plausible/analytics:assets/js/dashboard/nav-menu/filters-bar.test.tsx: width: 900,
plausible/analytics:test/plausible_web/components/billing/billing_test.exs: html = render_progress_bar(0, 0)
plausible/analytics:test/plausible_web/components/billing/billing_test.exs: assert html =~ "width: 0%"
plausible/analytics:tracker/test/fixtures/legacy-pageview-properties.html: <meta name="viewport" content="width=device-width, initial-scale=1.0" />
plausible/analytics:tracker/test/fixtures/legacy-pageview-properties.html: event-foo="bar"
plausible/analytics:tracker/test/fixtures/cookies-onetrust.html: <div class="ot-fltr-scrlcnt ot-pc-scrollbar">
plausible/analytics:tracker/test/fixtures/cookies-onetrust.html: style="position: absolute; top: -50000px; width: 100em"
plausible/analytics:tracker/test/fixtures/cookies-cookiebot.html: width="14"

--- Query 2: 'ghx gkoreli' (gh search code wraps in quotes = exact phrase, finds nothing) ---
```
