# Code search

Search for a multi-word query. Tests AND matching (ghx) vs exact-phrase matching (gh search code wraps in quotes).

## Results

| Command | Time (ms) | Input Tokens | Output Tokens | Output Lines |
|---------|-----------|--------------|---------------|--------------|
| `ghx search "bar width repo:plausible/analytics"` | 415ms | 13 | 360 | 20 |
| `gh search code "bar width repo:plausible/analytics"` | 359ms | 13 | 951 | 36 |
| `ghx search "ghx gkoreli"` | 341ms | 11 | 16 | 1 |
| `gh search code "ghx gkoreli"` | 320ms | 11 | 1 | 0 |

## `ghx search "bar width repo:plausible/analytics"`

Input tokens: 13 | Output tokens: 360 | Time: 415ms | Lines: 20 | Bytes: 1529

```
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
```

## `gh search code "bar width repo:plausible/analytics"`

Input tokens: 13 | Output tokens: 951 | Time: 359ms | Lines: 36 | Bytes: 3822

```
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
```

## `ghx search "ghx gkoreli"`

Input tokens: 11 | Output tokens: 16 | Time: 341ms | Lines: 1 | Bytes: 63

```
alphaleadership/npm-check docs/pending-db.json:pending-db.json
```

## `gh search code "ghx gkoreli"`

Input tokens: 11 | Output tokens: 1 | Time: 320ms | Lines: 0 | Bytes: 0

```

```

