# Zones fixtures: labelling sheet

14 cases for the zones experiment. Each has a zone: the neighbouring files a probe sees with `-zone`. In 10 the right answer depends on the zone (a convention, a duplicate, a neighbour's signature or units); 4 are controls (a messy zone around a fine target, a bug in a neighbour, a bug the zone is irrelevant to). **Labelled by the person on 2026-10-27, before any run**, judging with the zone visible: it is what the colony would know.

The labels below are my proposal. Change any you disagree with (edit `accept` in `fixtures.json`, or tell Claude), and mark each one ✅ when you agree. The question for each: *would a careful developer, seeing exactly what the model sees, give this label?* Accept several signals when several are fair, `none` when the code needs nothing.

Signals: 🌱 missing · 🧪 needs_test · 🐛 bug · 👀 review · ❓ unsure · 🧹 cleanup · 📝 needs_doc · 🧩 misfit · 🙋 human · none (see the README glossary).

| # | Fixture | Instrument | Zone | Proposed | Your label |
|---|---|---|---|---|---|
| 1 | zn-raw-throw | **model only** | orders | 🧩 | ✅ 🧩 |
| 2 | zn-local-date-format | **model only** | orders | 🧩 🧹 | ✅ 🧩 🧹 |
| 3 | zn-subject-among-signals | **model only** | cart | 🧩 | ✅ 🧩 |
| 4 | zn-fetch-bypasses-client | **model only** | invoices | 🧩 | ✅ 🧩 🐛 |
| 5 | zn-dup-price-format | duplication detector | cart | 🧹 | ✅ 🧹 |
| 6 | zn-dup-email-regex | duplication detector | account | 🧹 | ✅ 🧹 |
| 7 | zn-dup-mapper-sibling | duplication detector | invoices | 🧹 🧩 | ✅ 🧹 🧩 |
| 8 | zn-swapped-arguments | **model only** | orders | 🐛 | ✅ 🐛 |
| 9 | zn-cents-as-euros | **model only** | cart | 🐛 | ✅ 🐛 |
| 10 | zn-contract-changed-yesterday | **model only** | invoices | 🐛 👀 | ✅ 🐛 👀 |
| 11 | zn-clean-in-legacy-zone | all quiet | legacy reports | none | ✅ none |
| 12 | zn-follows-convention | all quiet | orders | none | ✅ none |
| 13 | zn-neighbour-has-bug | all quiet | cart | none | ✅ none |
| 14 | zn-target-own-bug | **model only** | cart | 🐛 | ✅ 🐛 🧪 |

## 1. zn-raw-throw

*throws and logs where every neighbour returns a Result and reports through ErrorsService*

**Target** `src/app/orders/order-history.service.ts#history` · **Instrument** model only · **Trail** `order-history.service.ts  ✅`

```ts
async history(id: string): Promise<OrderEvent[]> {
  const res = await fetch(`/v2/orders/${id}/history`);
  if (!res.ok) { console.error('history failed', res.status); throw new Error('history'); }
  return res.json();
}
```

**Facts (trusted)**

- files: order-history.service.ts, order-history.service.spec.ts
- tests: order-history.service.spec.ts, 2 tests, passing
- callers: order-detail.page.ts

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/orders/orders.service.ts`

```ts
list(): Observable<Result<Order[]>> {
  return this.api.get<OrderDto[]>('/v2/orders').pipe(map(ok(toOrders)), catchError(e => of(this.errors.report(e))));
}
```

`src/app/orders/order-export.service.ts`

```ts
export(id: string): Observable<Result<Blob>> {
  return this.api.getBlob(`/v2/orders/${id}/pdf`).pipe(map(ok), catchError(e => of(this.errors.report(e))));
}
```

**Proposed label:** 🧩

**Your label:** ✅ 🧩


## 2. zn-local-date-format

*formats a date by hand where the zone uses a shared formatDay helper*

**Target** `src/app/orders/order-card.component.ts#createdLabel` · **Instrument** model only · **Trail** `order-card.component.ts  ✅`

```ts
get createdLabel(): string {
  const d = this.order.createdAt;
  return `${d.getDate()}/${d.getMonth() + 1}/${d.getFullYear()}`;
}
```

**Facts (trusted)**

- files: order-card.component.ts, .html, .spec.ts
- tests: order-card.component.spec.ts, 3 tests, passing

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/orders/order-line.component.ts`

```ts
get shippedLabel(): string { return formatDay(this.line.shippedAt); }
```

`src/app/orders/order-history.component.ts`

```ts
label(e: OrderEvent): string { return `${formatDay(e.at)} · ${e.kind}`; }
```

`src/app/shared/dates.ts`

```ts
/** dd/MM/yyyy in the user's locale, the one date format of the app. */
export function formatDay(d: Date): string { return formatDate(d, 'dd/MM/yyyy', currentLocale()); }
```

**Proposed label:** 🧩 🧹

**Your label:** ✅ 🧩 🧹


## 3. zn-subject-among-signals

*keeps state in a BehaviorSubject where the whole zone uses Angular signals*

**Target** `src/app/cart/cart-badge.component.ts` · **Instrument** model only · **Trail** `cart-badge.component.ts  ✅`

```ts
export class CartBadgeComponent {
  private count$ = new BehaviorSubject(0);
  readonly count = this.count$.asObservable();
  constructor(cart: CartStore) { effect(() => this.count$.next(cart.lines().length)); }
}
```

**Facts (trusted)**

- files: cart-badge.component.ts, .html, .spec.ts
- tests: cart-badge.component.spec.ts, 1 test, passing

**Zone `cart`** (shown to probes only with `-zone`)

`src/app/cart/cart.store.ts`

```ts
export class CartStore {
  readonly lines = signal<CartLine[]>([]);
  readonly total = computed(() => this.lines().reduce((s, l) => s + l.cents, 0));
}
```

`src/app/cart/cart-summary.component.ts`

```ts
export class CartSummaryComponent {
  readonly store = inject(CartStore);
  readonly empty = computed(() => this.store.lines().length === 0);
}
```

**Proposed label:** 🧩

**Your label:** ✅ 🧩 (code fixed: the store has no lines$, the target now reads cart.lines() in an effect)


## 4. zn-fetch-bypasses-client

*calls fetch on a v1 URL where the zone goes through ApiClient on v2*

**Target** `src/app/invoices/invoice-download.ts#download` · **Instrument** model only · **Trail** `invoice-download.ts  (no signals)`

```ts
export async function download(id: string): Promise<Blob> {
  const r = await fetch('/api/v1/invoices/' + id + '/pdf', { credentials: 'include' });
  return r.blob();
}
```

**Facts (trusted)**

- files: invoice-download.ts
- tests: none for invoice-download.ts
- callers: invoice-list.component.ts

**Zone `invoices`** (shown to probes only with `-zone`)

`src/app/invoices/invoices.service.ts`

```ts
list(): Observable<Invoice[]> { return this.api.get<InvoiceDto[]>('/v2/invoices').pipe(map(toInvoices)); }
pdf(id: string): Observable<Blob> { return this.api.getBlob(`/v2/invoices/${id}/pdf`); }
```

`src/app/core/api-client.ts`

```ts
/** Every HTTP call of the app: base URL, auth header, retries, error mapping. */
export class ApiClient { get<T>(path: string) { /* ... */ } getBlob(path: string) { /* ... */ } }
```

**Proposed label:** 🧩

**Your label:** ✅ 🧩 🐛


## 5. zn-dup-price-format

*re-implements the zone's money formatter*

**Target** `src/app/cart/cart-line.component.ts#price` · **Instrument** duplication detector · **Trail** `cart-line.component.ts  ✅`

```ts
get price(): string {
  return (this.line.cents / 100).toFixed(2).replace('.', ',') + ' €';
}
```

**Facts (trusted)**

- files: cart-line.component.ts, .html, .spec.ts
- tests: cart-line.component.spec.ts, 2 tests, passing

**Zone `cart`** (shown to probes only with `-zone`)

`src/app/shared/money.ts`

```ts
/** 1999 -> '19,99 €'. The one money format of the app. */
export function formatEuros(cents: number): string { return (cents / 100).toFixed(2).replace('.', ',') + ' €'; }
```

`src/app/cart/cart-summary.component.ts`

```ts
readonly totalLabel = computed(() => formatEuros(this.store.total()));
```

**Proposed label:** 🧹

**Your label:** ✅ 🧹


## 6. zn-dup-email-regex

*copies the email check that the zone's validators already export*

**Target** `src/app/account/signup.component.ts#emailOk` · **Instrument** duplication detector · **Trail** `signup.component.ts  ✅`

```ts
emailOk(v: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(v.trim());
}
```

**Facts (trusted)**

- files: signup.component.ts, .html, .spec.ts
- tests: signup.component.spec.ts, 4 tests, passing

**Zone `account`** (shown to probes only with `-zone`)

`src/app/account/validators.ts`

```ts
export const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;
export const emailValidator: ValidatorFn = c => EMAIL.test((c.value ?? '').trim()) ? null : { email: true };
```

`src/app/account/profile.component.ts`

```ts
form = new FormGroup({ email: new FormControl('', [Validators.required, emailValidator]) });
```

**Proposed label:** 🧹

**Your label:** ✅ 🧹


## 7. zn-dup-mapper-sibling

*maps the same DTO a second time, inline, instead of using the zone's mapper*

**Target** `src/app/invoices/invoice-list.component.ts#rows` · **Instrument** duplication detector · **Trail** `invoice-list.component.ts  ✅`

```ts
rows(dtos: InvoiceDto[]): Invoice[] {
  return dtos.map(d => ({ id: d.id, number: d.number, totalCents: d.total_cents, issuedAt: new Date(d.issued_at) }));
}
```

**Facts (trusted)**

- files: invoice-list.component.ts, .html, .spec.ts
- tests: invoice-list.component.spec.ts, 2 tests, passing

**Zone `invoices`** (shown to probes only with `-zone`)

`src/app/invoices/invoice.mapper.ts`

```ts
export function toInvoice(d: InvoiceDto): Invoice {
  return { id: d.id, number: d.number, totalCents: d.total_cents, issuedAt: new Date(d.issued_at) };
}
export const toInvoices = (ds: InvoiceDto[]) => ds.map(toInvoice);
```

**Proposed label:** 🧹 🧩

**Your label:** ✅ 🧹 🧩 (code fixed: Invoice has totalCents, the target now repeats the mapper inline)


## 8. zn-swapped-arguments

*passes two string ids in the wrong order to a neighbour's function*

**Target** `src/app/orders/order-actions.ts#remind` · **Instrument** model only · **Trail** `order-actions.ts  ✅`

```ts
remind(order: Order): Observable<void> {
  return this.notifications.notify(order.customerId, order.id);
}
```

**Facts (trusted)**

- files: order-actions.ts, order-actions.spec.ts
- tests: order-actions.spec.ts, 1 test, passing (mocks notifications)

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/notifications/notifications.service.ts`

```ts
/** Sends the reminder for one order to its customer. */
notify(orderId: string, customerId: string): Observable<void> { /* ... */ }
```

**Proposed label:** 🐛

**Your label:** ✅ 🐛


## 9. zn-cents-as-euros

*displays a value in cents as if it were euros*

**Target** `src/app/cart/cart-total.component.ts#label` · **Instrument** model only · **Trail** `cart-total.component.ts  ✅`

```ts
get label(): string {
  return this.store.total().toFixed(2) + ' €';
}
```

**Facts (trusted)**

- files: cart-total.component.ts, .html, .spec.ts
- tests: cart-total.component.spec.ts, 1 test, passing (total mocked as 12)

**Zone `cart`** (shown to probes only with `-zone`)

`src/app/cart/cart.store.ts`

```ts
export class CartStore {
  readonly lines = signal<CartLine[]>([]);
  /** Total in cents. */
  readonly total = computed(() => this.lines().reduce((s, l) => s + l.cents, 0));
}
```

**Proposed label:** 🐛

**Your label:** ✅ 🐛


## 10. zn-contract-changed-yesterday

*consumer still reads a field the neighbour renamed yesterday*

**Target** `src/app/invoices/invoice-badge.component.ts#overdue` · **Instrument** model only · **Trail** `invoice-badge.component.ts  ✅`

```ts
get overdue(): boolean {
  return (this.invoice as any).due_date < new Date();
}
```

**Facts (trusted)**

- files: invoice-badge.component.ts, .html, .spec.ts
- tests: invoice-badge.component.spec.ts, 1 test, passing
- git: invoice.mapper.ts changed yesterday (dueDate added, due_date removed), not reviewed

**Zone `invoices`** (shown to probes only with `-zone`)

`src/app/invoices/invoice.mapper.ts`

```ts
export function toInvoice(d: InvoiceDto): Invoice {
  return { id: d.id, number: d.number, totalCents: d.total_cents, issuedAt: new Date(d.issued_at), dueDate: new Date(d.due_at) };
}
```

**Proposed label:** 🐛 👀

**Your label:** ✅ 🐛 👀


## 11. zn-clean-in-legacy-zone

*clean, tested target in a zone full of leftovers*

**Target** `src/app/legacy/report-title.ts#reportTitle` · **Instrument** all quiet · **Trail** `report-title.ts  ✅`

```ts
/** Title shown on the PDF report: client name and period, e.g. 'Dupont · 2026-Q3'. */
export function reportTitle(client: string, year: number, quarter: 1 | 2 | 3 | 4): string {
  return `${client.trim()} · ${year}-Q${quarter}`;
}
```

**Facts (trusted)**

- files: report-title.ts, report-title.spec.ts
- tests: report-title.spec.ts, 3 tests, passing
- last change: 3 months ago, reviewed

**Zone `legacy reports`** (shown to probes only with `-zone`)

`src/app/legacy/report.ts`

```ts
export function r(d: any, x: any) {
  console.log('REPORT', d);
  // const old = build(d);
  return build2(d, x || {});
}
```

`src/app/legacy/report-utils.ts`

```ts
export function fmt(a: any) { return a + ''; }
export function fmt2(a: any) { return '' + a; }
```

**Proposed label:** none

**Your label:** ✅ none


## 12. zn-follows-convention

*target follows the zone's Result convention exactly*

**Target** `src/app/orders/order-notes.service.ts#notes` · **Instrument** all quiet · **Trail** `order-notes.service.ts  ✅`

```ts
notes(id: string): Observable<Result<OrderNote[]>> {
  return this.api.get<OrderNoteDto[]>(`/v2/orders/${id}/notes`).pipe(map(ok(toNotes)), catchError(e => of(this.errors.report(e))));
}
```

**Facts (trusted)**

- files: order-notes.service.ts, order-notes.service.spec.ts
- tests: order-notes.service.spec.ts, 2 tests, passing
- last change: 1 month ago, reviewed

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/orders/orders.service.ts`

```ts
list(): Observable<Result<Order[]>> {
  return this.api.get<OrderDto[]>('/v2/orders').pipe(map(ok(toOrders)), catchError(e => of(this.errors.report(e))));
}
```

`src/app/orders/order-export.service.ts`

```ts
export(id: string): Observable<Result<Blob>> {
  return this.api.getBlob(`/v2/orders/${id}/pdf`).pipe(map(ok), catchError(e => of(this.errors.report(e))));
}
```

**Proposed label:** none

**Your label:** ✅ none


## 13. zn-neighbour-has-bug

*target is fine; a neighbour in the zone has an off-by-one*

**Target** `src/app/cart/cart-count.ts#lineCount` · **Instrument** all quiet · **Trail** `cart-count.ts  ✅`

```ts
/** Number of distinct products in the cart. */
export function lineCount(lines: CartLine[]): number {
  return new Set(lines.map(l => l.productId)).size;
}
```

**Facts (trusted)**

- files: cart-count.ts, cart-count.spec.ts
- tests: cart-count.spec.ts, 3 tests, passing

**Zone `cart`** (shown to probes only with `-zone`)

`src/app/cart/cart-pages.ts`

```ts
export function page(lines: CartLine[], n: number, size = 10) {
  return lines.slice(n * size, n * size + size + 1);
}
```

**Proposed label:** none

**Your label:** ✅ none


## 14. zn-target-own-bug

*the bug is in the target itself; neighbours are fine*

**Target** `src/app/cart/free-shipping.ts#isFreeShipping` · **Instrument** model only · **Trail** `free-shipping.ts  ✅`

```ts
/** Free shipping from 50 € included. */
export function isFreeShipping(totalCents: number): boolean {
  return totalCents > 5000;
}
```

**Facts (trusted)**

- files: free-shipping.ts, free-shipping.spec.ts
- tests: free-shipping.spec.ts, 2 tests (0 and 10000 cents), passing

**Zone `cart`** (shown to probes only with `-zone`)

`src/app/cart/cart.store.ts`

```ts
export class CartStore {
  readonly lines = signal<CartLine[]>([]);
  /** Total in cents. */
  readonly total = computed(() => this.lines().reduce((s, l) => s + l.cents, 0));
}
```

**Proposed label:** 🐛

**Your label:** ✅ 🐛 🧪

