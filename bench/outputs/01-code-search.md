# Code search: ghx vs gh

Compare ghx search (AND + text_matches + token protection) vs gh search code (exact phrase, no context).
ghx sends unquoted AND queries, shows matching lines, truncates to 200 chars, reports count on stderr.
gh search code silently wraps in quotes (exact phrase), shows only paths, no match context.

## Results

| Command | Time (ms) | Input Tokens | Output Tokens | Output Lines |
|---------|-----------|--------------|---------------|--------------|
| `ghx search "bar width repo:plausible/analytics"` | 711ms | 13 | 927 | 20 |
| `gh search code "bar width repo:plausible/analytics"` | 388ms | 13 | 951 | 36 |
| `ghx search "ghx gkoreli"` | 861ms | 11 | 73 | 1 |
| `gh search code "ghx gkoreli"` | 410ms | 11 | 1 | 0 |
| `ghx search "addClass repo:jquery/jquery"` | 794ms | 10 | 395 | 8 |
| `gh search code "addClass repo:jquery/jquery"` | 441ms | 10 | 442 | 14 |
| `ghx search "jQuery.fn.extend filename:jquery.min.js"` | 2956ms | 13 | 1602 | 30 |
| `gh search code "jQuery.fn.extend filename:jquery.min.js"` | 766ms | 13 | 60200 | 69 |
| `ghx search "useState language:typescript"` | 858ms | 9 | 1784 | 30 |
| `gh search code "useState language:typescript"` | 420ms | 9 | 1101 | 41 |

## `ghx search "bar width repo:plausible/analytics"`

Input tokens: 13 | Output tokens: 927 | Time: 711ms | Lines: 20 | Bytes: 3454

**stderr:**
```
20 results (showing 20)
⚠ Lines truncated to 200 chars (use --full for complete fragments)
```

**stdout:**
```
plausible/analytics assets/js/dashboard/stats/bar.js: return (count / maxVal) * 100 } export default function Bar({ count, all, bg,
plausible/analytics assets/js/dashboard/nav-menu/filters-bar.tsx: ) => (element === null ? null : element.getBoundingClientRect().width) type VisibilityState = { width: number visibleCount: number }
plausible/analytics assets/js/dashboard/nav-menu/top-bar.tsx: import { FilterMenu } from './filter-menu' import { FiltersBar } from './filters-bar' import { DashboardPeriodPicker } from './query-periods/dashboard-period-picker'
plausible/analytics assets/js/dashboard/extra/funnel.js: …lStyle = color1 c.strokeStyle = color2 c.fillRect(0, 0, shape.width, shape.height) c.beginPath() c.moveTo(2, 0)
plausible/analytics assets/js/dashboard/stats/reports/list.tsx: return ( <div className="grow w-full overflow-hidden"> <Bar maxWidthDeduction={undefined} count={listItem[metricToPlot]} all={state.list}
plausible/analytics lib/plausible_web/components/billing/billing.ex: |> assign(:percentage, percentage) |> assign(:color_class, progress_bar_color_from_percentage(percentage, assigns.limit))
plausible/analytics assets/css/app.css: …-webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale; width: 100vw; /* Prevents content from jumping when scrollbar is added/removed due to vertical overflow */ overflow-x: hidden;…
plausible/analytics CHANGELOG.md: …Allow running the container with arbitrary UID plausible/analytics#2986 - Fix `width=manual` in embedded dashboards plausible/analytics#3910 - Fix URL escaping when pipes are used in UTM tags plausib…
plausible/analytics lib/plausible_web/templates/settings/api_keys.html.heex: </.button_link> </.filter_bar>
plausible/analytics lib/plausible_web/live/goal_settings/list.ex: </PrimaDropdown.dropdown> </.filter_bar> <% end %>
... (20 lines total)
```

## `gh search code "bar width repo:plausible/analytics"`

Input tokens: 13 | Output tokens: 951 | Time: 388ms | Lines: 36 | Bytes: 3822


**stdout:**
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
... (36 lines total)
```

## `ghx search "ghx gkoreli"`

Input tokens: 11 | Output tokens: 73 | Time: 861ms | Lines: 1 | Bytes: 188

**stderr:**
```
1 results (showing 1)
```

**stdout:**
```
alphaleadership/npm-check docs/pending-db.json: "timestamp": "2026-03-08T16:51:54.328Z" }, { "packageName": "@gkoreli/ghx", "version": "latest", "timestamp": "2026-03-08T16:51:43.330Z" },
```

## `gh search code "ghx gkoreli"`

Input tokens: 11 | Output tokens: 1 | Time: 410ms | Lines: 0 | Bytes: 0


**stdout:**
```

```

## `ghx search "addClass repo:jquery/jquery"`

Input tokens: 10 | Output tokens: 395 | Time: 794ms | Lines: 8 | Bytes: 1417

**stderr:**
```
8 results (showing 8)
⚠ Lines truncated to 200 chars (use --full for complete fragments)
```

**stdout:**
```
jquery/jquery src/attributes/classes.js: } jQuery.fn.extend( { addClass: function( value ) { var classNames, cur, curValue, className, i, finalValue; if ( typeof value === "function" ) {
jquery/jquery changelog.md: …ery/commit/ed306c0261ab63746040e5d58bb4477c3069a427)) - Skip falsy values in `addClass( array )`, compress code ([#4998](https://github.com/jquery/jquery/issues/4998), [a338b407](https://github.com/jq…
jquery/jquery test/unit/basic.js: location.href.replace( /\#.*$/, "" ) + "#5", ".prop getter/setter" ); a.addClass( "abc def ghj" ).removeClass( "def ghj" ); assert.strictEqual( a.hasClass( "abc" ), true, ".(add|remove|has)Class, cla…
jquery/jquery test/unit/attributes.js: j = jQuery( "#nonnodes" ).contents(); j.addClass( valueObj( "asdf" ) ); assert.ok( j.hasClass( "asdf" ), "Check node,textnode,comment for addClass" );
jquery/jquery test/unit/css.js: …), cascadeHiddenDetached = jQuery( "<p><a></a></p>" ).find( "*" ).addBack().addClass( "hidden" );
jquery/jquery test/unit/manipulation.js: div.addClass( "test" );
jquery/jquery test/unit/effects.js: testClass = jQuery.makeTest( "Overflow and Display" ) .addClass( "overflow inline" ), testStyle = jQuery.makeTest( "Overflow and Display (inline style)" )
jquery/jquery test/data/jquery-3.7.1.js: } jQuery.fn.extend( { addClass: function( value ) { var classNames, cur, curValue, className, i, finalValue; if ( isFunction( value ) ) {
```

## `gh search code "addClass repo:jquery/jquery"`

Input tokens: 10 | Output tokens: 442 | Time: 441ms | Lines: 14 | Bytes: 1637


**stdout:**
```
jquery/jquery:src/attributes/classes.js: addClass: function( value ) {
jquery/jquery:changelog.md: - Skip falsy values in `addClass( array )`, compress code ([#4998](https://github.com/jquery/jquery/issues/4998), [a338b407](https://github.com/jquery/jquery/commit/a338b407f2479f82df40635055effc163835183f))
jquery/jquery:test/unit/basic.js: a.addClass( "abc def ghj" ).removeClass( "def ghj" );
jquery/jquery:test/unit/attributes.js: j.addClass( valueObj( "asdf" ) );
jquery/jquery:test/unit/attributes.js: assert.ok( j.hasClass( "asdf" ), "Check node,textnode,comment for addClass" );
jquery/jquery:test/unit/attributes.js: $set.addClass( "test" ).addClass( "foo" ).addClass( "bar" );
jquery/jquery:test/unit/css.js: cascadeHiddenDetached = jQuery( "<p><a></a></p>" ).find( "*" ).addBack().addClass( "hidden" );
jquery/jquery:test/unit/css.js: $elem = jQuery( "<div>" ).addClass( "test__customProperties" )
jquery/jquery:test/unit/manipulation.js: div.addClass( "test" );
jquery/jquery:test/unit/manipulation.js: assert.strictEqual( jQuery( "<div></div>" ).clone().addClass( "test" ).appendTo( "<div></div>" ).end().end().hasClass( "test" ), false, "Check jQuery.fn.appendTo after jQuery.clone" );
... (14 lines total)
```

## `ghx search "jQuery.fn.extend filename:jquery.min.js"`

Input tokens: 13 | Output tokens: 1602 | Time: 2956ms | Lines: 30 | Bytes: 5995

**stderr:**
```
90 results (showing 30)
⚠ Lines truncated to 200 chars (use --full for complete fragments)
```

**stdout:**
```
james-fray/local-cdn src/data/resources/jquery/1.2.6/jquery.min.js.dec: …lem.parentNode.removeChild(elem);}function now(){return+new Date;}jQuery.extend=jQuery.fn.extend=function(){var target=arguments[0]||{},i=1,length=arguments.length,deep=false,options;if(target.constru…
petterobam/Online-Powerpoint template.html/jquery.min.js.txt: jQuery.extend = jQuery.fn.extend = function() { var src, copyIsArray, copy, name, options, clone,
wenpi/bems bems_v4/WebContent/js/plugin/.svn/text-base/jquery.min.js.svn-base: jQuery.extend = jQuery.fn.extend = function() { var options, name, src, copy, copyIsArray, clone,
feihb123/flight WebContent/Places_files/jquery.min.js.下载: jQuery.fn.extend( { has: function( target ) {
Islem618/AITISAL_html FR/src/js/jquery.min.js.js: jQuery.fn.extend({ has: function (target) {
Yensid10/Multipurpose-Flask-Website-System-Project docs/jquery.min.js.html: jQuery.fn.extend( { has: function( target ) {
yelayou/egma files/scripts/jquery.min.js.download: …Until|All))/,guaranteedUnique={children:true,contents:true,next:true,prev:true};jQuery.fn.extend({has:function(target){var i,targets=jQuery(target,this),len=targets.length;return this.filter(function(…
nicoletsai0603/eeis2 系統公告 _ 環境教育探索館_files/jquery.min.js.下載: jQuery.fn.extend( { has: function( target ) {
roiding/coins 纪念币预约_files/jquery.min.js.下载: jQuery.extend = jQuery.fn.extend = function() { var src, copyIsArray, copy, name, options, clone,
WISVCH/chipcie-website static/archive/2021/dapc/BAPC Preliminaries 2021_files/jquery.min.js.download: jQuery.fn.extend( { has: function( target ) {
... (30 lines total)
```

## `gh search code "jQuery.fn.extend filename:jquery.min.js"`

Input tokens: 13 | Output tokens: 60200 | Time: 766ms | Lines: 69 | Bytes: 236661


**stdout:**
```
james-fray/local-cdn:src/data/resources/jquery/1.2.6/jquery.min.js.dec: jQuery.globalEval(elem.text||elem.textContent||elem.innerHTML||"");if(elem.parentNode)elem.parentNode.removeChild(elem);}function now(){return+new Date;}jQuery.extend=jQuery.fn.extend=function(){var target=arguments[0]||{},i=1,length=arguments.length,deep=false,options;if(target.constructor==Boolean){deep=target;target=arguments[1]||{};i=2;}if(typeof target!="object"&&typeof target!="function")target={};if(length==i){target=
james-fray/local-cdn:src/data/resources/jquery/1.2.6/jquery.min.js.dec: for(handler in events[type])if(!parts[1]||events[type][handler].type==parts[1])delete events[type][handler];for(ret in events[type])break;if(!ret){if(!jQuery.event.special[type]||jQuery.event.special[type].teardown.call(elem)===false){if(elem.removeEventListener)elem.removeEventListener(type,jQuery.data(elem,"handle"),false);else if(elem.detachEvent)elem.detachEvent("on"+type,jQuery.data(elem,"handle"));}ret=null;delete even
james-fray/local-cdn:src/data/resources/jquery/1.2.6/jquery.min.js.dec: jQuery.readyList.push(function(){return fn.call(this,jQuery);});return this;}});jQuery.extend({isReady:false,readyList:[],ready:function(){if(!jQuery.isReady){jQuery.isReady=true;if(jQuery.readyList){jQuery.each(jQuery.readyList,function(){this.call(document);});jQuery.readyList=null;}jQuery(document).triggerHandler("ready");}}});var readyBound=false;function bindReady(){if(readyBound)return;readyBound=true;if(document.addEv
petterobam/Online-Powerpoint:template.html/jquery.min.js.txt: jQuery.extend = jQuery.fn.extend = function() {
petterobam/Online-Powerpoint:template.html/jquery.min.js.txt: jQuery.fn.extend({
wenpi/bems:bems_v4/WebContent/js/plugin/.svn/text-base/jquery.min.js.svn-base: jQuery.extend = jQuery.fn.extend = function() {
wenpi/bems:bems_v4/WebContent/js/plugin/.svn/text-base/jquery.min.js.svn-base: jQuery.fn.extend({
feihb123/flight:WebContent/Places_files/jquery.min.js.下载: jQuery.fn.extend( {
feihb123/flight:WebContent/Places_files/jquery.min.js.下载: jQuery.fn.extend( {
Islem618/AITISAL_html:FR/src/js/jquery.min.js.js: jQuery.fn.extend({
... (69 lines total)
```

## `ghx search "useState language:typescript"`

Input tokens: 9 | Output tokens: 1784 | Time: 858ms | Lines: 30 | Bytes: 6565

**stderr:**
```
201472 results (showing 30)
⚠ Query too broad — add repo:, language:, or path: to narrow
⚠ Lines truncated to 200 chars (use --full for complete fragments)
```

**stdout:**
```
staltz/use-profunctor-state react.d.ts: …export type SetState<T> = (updater: Updater<T>) => void; export function useState<T = any>(initial: T): [T, SetState<T>]; export function useMemo<T = any>(factory: () => T, args: Array<any>): T; expo…
littledivy/wgui hooks.ts: * ) * ``` */ export function useState<T>(initialValue: T): [T, (T: T) => void] { const hook = hooks[currentHook] ?? initialValue; const i = currentHook; const setState = (value: any) => {
zstackio/zstack-dashboard ts/zone.ts: state: string = 'All'; buttonName: string; useState() : string { if (this.state == 'All') { this.buttonName = 'state:all'; } else if (this.state == 'Enabled') {
shallinta/vue3-hooks index.ts: return typeof f === 'function'; } export const useState = <T>(defaultValue: T) => { if (isObject(defaultValue)) { return useStateObj(defaultValue); }
PutziSan/react-factory-hooks index.d.ts: declare type FactoryState = { [key: string]: any; }; export declare function useState<T>(initialValue: T): UseStateReturnValue<T>; export declare function useEffect<T extends any[]>(effect: Effect<T>,…
MatthiasKainer/lit-element-state-decoupler state.ts: …wClone } from "./clone"; import { withState } from "./decorator"; export const useState = <T>(element: LitLikeElement, defaultValue: T, options: StateOptions = {}): State<T> => { let state = shallowCl…
peterboyer/pb.adt README.ts: …ework (e.g. React) rendering all state cases. //> type Element = any; //- const useState = <T>(_t: T) => ({}) as [T, (t: T) => void]; //- const useEffect = (_cb: () => void, _deps: never[]) => undefin…
sfishel18/rxjs-fiddle-frontend types.ts: …tends object>(component: React.FC<T>): MemoizedFC<T>; function useState<T>(initialState: T | (() => T)): MemoizedStateTuple<T>; } }
virtualstate/eerie src/h.ts: [HookState]: ComponentHookState; useState<T>(defaultValue?: UseStateDefault<T>): UseStateReturn<T>; useEffect(fn: EffectFn, dependencies?: Dependencies): void;
SisyphusZheng/me jsx.d.ts: } declare module "preact/hooks" { export function useState<T>( initialState: T | (() => T) ): [T, (value: T | ((prevState: T) => T)) => void]; export function useEffect(
... (30 lines total)
```

## `gh search code "useState language:typescript"`

Input tokens: 9 | Output tokens: 1101 | Time: 420ms | Lines: 41 | Bytes: 3713


**stdout:**
```
staltz/use-profunctor-state:react.d.ts: export function useState<T = any>(initial: T): [T, SetState<T>];
littledivy/wgui:hooks.ts: export function useState<T>(initialValue: T): [T, (T: T) => void] {
zstackio/zstack-dashboard:ts/zone.ts: useState() : string {
shallinta/vue3-hooks:index.ts: export const useState = <T>(defaultValue: T) => {
shallinta/vue3-hooks:index.ts: return useStateObj(defaultValue);
PutziSan/react-factory-hooks:index.d.ts: export declare function useState<T>(initialValue: T): UseStateReturnValue<T>;
MatthiasKainer/lit-element-state-decoupler:state.ts: export const useState = <T>(element: LitLikeElement, defaultValue: T, options: StateOptions = {}): State<T> => {
peterboyer/pb.adt:README.ts: const useState = <T>(_t: T) => ({}) as [T, (t: T) => void]; //-
sfishel18/rxjs-fiddle-frontend:types.ts: function useState<T>(initialState: T | (() => T)): MemoizedStateTuple<T>;
virtualstate/eerie:src/h.ts: useState<T>(defaultValue?: UseStateDefault<T>): UseStateReturn<T>;
... (41 lines total)
```

