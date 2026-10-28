# Held-out fixtures: labelling sheet

16 new cases for the held-out test. **Labelled by the person on 2026-10-27, before any run.** Four also carry a zone, used only by the zones experiment; the held-out test itself runs without zones.

The labels below are my proposal. Change any you disagree with (edit `accept` in `fixtures.json`, or tell Claude), and mark each one ✅ when you agree. The question for each: *would a careful developer, seeing exactly what the model sees, give this label?* Accept several signals when several are fair, `none` when the code needs nothing.

Signals: 🌱 missing · 🧪 needs_test · 🐛 bug · 👀 review · ❓ unsure · 🧹 cleanup · 📝 needs_doc · 🧩 misfit · 🙋 human · none (see the README glossary).

| # | Fixture | Instrument | Zone | Proposed | Your label |
|---|---|---|---|---|---|
| 1 | hx-first-match-undefined | **model only** | orders | 🐛 | ✅ 🐛 🧪 |
| 2 | hx-date-utc-shift | **model only** | – | 🐛 | ✅ 🐛 |
| 3 | hx-float-total-equality | **model only** | – | 🐛 | ✅ 🐛 |
| 4 | hx-magic-codes | **model only** | – | ❓ 📝 | ✅ ❓ 📝 🧪 |
| 5 | hx-boolean-flags | **model only** | orders | ❓ 📝 | ✅ ❓ 📝 |
| 6 | hx-silent-catch | lint | – | 🐛 🧹 | ✅ 🐛 🧹 |
| 7 | hx-interval-no-teardown | lint | – | 🐛 🧩 | ✅ 🐛 🧩 |
| 8 | hx-clean-slugify | all quiet | – | none | ✅ none |
| 9 | hx-clean-mapper | all quiet | – | none | ✅ none |
| 10 | hx-untested-cancel | coverage | – | 🧪 | ✅ 🧪 |
| 11 | hx-missing-guard | compiler | – | 🌱 | ✅ 🌱 |
| 12 | hx-fresh-refund-rounding | git recency + path | – | 👀 🐛 | ✅ 👀 🙋 |
| 13 | hx-state-copy-in-component | plot proof | orders | 🧩 | ✅ 🧩 |
| 14 | hx-consent-wording | path rule | – | 🙋 | ✅ 🙋 👀 |
| 15 | hx-query-builder-undocumented | lint + fan-in | – | 📝 | ✅ 📝 |
| 16 | hx-twin-mappers | duplication detector | invoices | 🧹 | ✅ 🧹 |

## 1. hx-first-match-undefined

*reads a property of the first filter match, which may not exist*

**Target** `src/app/orders/order-owner.ts#ownerName` · **Instrument** model only · **Trail** `order-owner.ts  ✅`

```ts
export function ownerName(order: Order, users: User[]): string {
  const owner = users.filter(u => u.id === order.ownerId)[0];
  return owner.firstName + ' ' + owner.lastName;
}
```

**Facts (trusted)**

- files: order-owner.ts, order-owner.spec.ts
- tests: order-owner.spec.ts, 2 tests, passing (both with an owner present)
- callers: order-card.component.ts, order-export.service.ts
- orders can be created by deleted users (users-cleanup job, see git log)

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/users/users-cleanup.job.ts`

```ts
/** Deletes users inactive for 3 years. Their orders are kept. */
export async function cleanup(): Promise<void> { /* ... */ }
```

`src/app/orders/order-card.component.ts`

```ts
get owner(): string { return ownerName(this.order, this.users); }
```

**Proposed label:** 🐛

**Your label:** ✅ 🐛 🧪


## 2. hx-date-utc-shift

*parses an ISO date as UTC then reads the local day: one day early west of Greenwich*

**Target** `src/app/orders/delivery-date.ts#deliveryDay` · **Instrument** model only · **Trail** `delivery-date.ts  ✅`

```ts
// Day of the month the delivery is planned for, shown on the order card.
export function deliveryDay(isoDate: string): number {
  return new Date(isoDate).getDate();   // isoDate looks like '2026-03-01'
}
```

**Facts (trusted)**

- files: delivery-date.ts, delivery-date.spec.ts
- tests: delivery-date.spec.ts, 1 test, passing (CI runs in UTC)
- users: France and Canada (Quebec)

**Proposed label:** 🐛

**Your label:** ✅ 🐛


## 3. hx-float-total-equality

*compares a sum of float prices with ===*

**Target** `src/app/cart/cart-total.ts#isFree` · **Instrument** model only · **Trail** `cart-total.ts  ✅`

```ts
export function isFree(lines: { price: number; discount: number }[]): boolean {
  const total = lines.reduce((s, l) => s + l.price - l.discount, 0);
  return total === 0;
}
```

**Facts (trusted)**

- files: cart-total.ts, cart-total.spec.ts
- tests: cart-total.spec.ts, 2 tests, passing (integer prices only)
- prices come from the API in euros with cents, e.g. 19.9

**Proposed label:** 🐛

**Your label:** ✅ 🐛


## 4. hx-magic-codes

*retry decision on unexplained numeric codes with one-letter names*

**Target** `src/app/sync/sync-client.ts#r` · **Instrument** model only · **Trail** `sync-client.ts  (no signals)`

```ts
export function r(e: { c: number }, n: number, f: () => Promise<void>): Promise<void> {
  if ([7, 13, 42].includes(e.c) && n > 0) return f().catch(x => r(x, n - 1, f));
  return Promise.reject(e);
}
```

**Facts (trusted)**

- files: sync-client.ts
- tests: none for sync-client.ts
- callers: 3 (sync.service.ts)

**Proposed label:** ❓ 📝

**Your label:** ✅ ❓ 📝 🧪


## 5. hx-boolean-flags

*a save call whose three boolean arguments cannot be read at the call site*

**Target** `src/app/orders/order-editor.component.ts#onConfirm` · **Instrument** model only · **Trail** `order-editor.component.ts  ✅`

```ts
onConfirm(): void {
  this.orders.save(this.order, true, false, true).subscribe(() => this.close());
}
// in orders.service.ts:
// save(order: Order, a: boolean, b: boolean, c: boolean): Observable<void>
```

**Facts (trusted)**

- files: order-editor.component.ts, .html, .spec.ts, orders.service.ts
- tests: order-editor.component.spec.ts, 3 tests, passing
- orders.service.ts#save has no documentation; 9 callers pass different flag combinations

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/orders/orders.service.ts`

```ts
save(order: Order, a: boolean, b: boolean, c: boolean): Observable<void> {
  return this.api.put(`/v2/orders/${order.id}`, { ...toDto(order), notify: a, draft: b, lock: c });
}
```

`src/app/orders/order-quick-edit.component.ts`

```ts
onSave(): void { this.orders.save(this.order, false, true, false).subscribe(); }
```

**Proposed label:** ❓ 📝

**Your label:** ✅ ❓ 📝


## 6. hx-silent-catch

*an empty catch swallows a failed payment confirmation*

**Target** `src/app/checkout/confirm.service.ts#confirm` · **Instrument** lint · **Trail** `confirm.service.ts  ✅`

```ts
async confirm(id: string): Promise<void> {
  try {
    await this.api.post(`/payments/${id}/confirm`, {});
  } catch {}
  this.router.navigate(['/thank-you']);
}
```

**Facts (trusted)**

- files: confirm.service.ts, confirm.service.spec.ts
- tests: confirm.service.spec.ts, 1 test, passing (success path only)
- lint: no-empty reports confirm.service.ts line 5

**Proposed label:** 🐛 🧹

**Your label:** ✅ 🐛 🧹


## 7. hx-interval-no-teardown

*component subscribes to an interval and never unsubscribes, against a capped practice cell*

**Target** `src/app/orders/live-status.component.ts#ngOnInit` · **Instrument** lint · **Trail** `live-status.component.ts  ✅`

**Honey**

```
🍯 A component that subscribes in ngOnInit unsubscribes in ngOnDestroy.
   test: lint rule no-leaked-subscription
```

```ts
export class LiveStatusComponent implements OnInit {
  status = '';
  constructor(private orders: OrdersService) {}
  ngOnInit(): void {
    interval(5000).pipe(switchMap(() => this.orders.status(this.id))).subscribe(s => this.status = s);
  }
  @Input() id = '';
}
```

**Facts (trusted)**

- files: live-status.component.ts, .html, .spec.ts
- tests: live-status.component.spec.ts, 1 test, passing
- lint: no-leaked-subscription reports live-status.component.ts line 5

**Proposed label:** 🐛 🧩

**Your label:** ✅ 🐛 🧩


## 8. hx-clean-slugify

*small pure function with tests and a doc comment*

**Target** `src/app/shared/slugify.ts#slugify` · **Instrument** all quiet · **Trail** `slugify.ts  ✅ 🔒`

```ts
/** Lower-case, accent-free, dash-separated form of a title, for URLs. */
export function slugify(title: string): string {
  return title
    .normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .toLowerCase().trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}
```

**Facts (trusted)**

- files: slugify.ts, slugify.spec.ts
- tests: slugify.spec.ts, 6 tests (accents, spaces, symbols, empty, leading and trailing dashes), passing
- last change: 5 months ago

**Proposed label:** none

**Your label:** ✅ none


## 9. hx-clean-mapper

*plain DTO mapping with a test, following the plot*

**Target** `src/app/orders/order.mapper.ts#toOrder` · **Instrument** all quiet · **Trail** `order.mapper.ts  ✅`

**Garden**

```
🍄 API DTOs are mapped to domain types in *.mapper.ts files; components never see a DTO.
   Proof: lint rule no-dto-in-components
```

```ts
export function toOrder(dto: OrderDto): Order {
  return {
    id: dto.id,
    createdAt: new Date(dto.created_at),
    lines: dto.lines.map(toOrderLine),
    status: dto.status,
  };
}
```

**Facts (trusted)**

- files: order.mapper.ts, order.mapper.spec.ts
- tests: order.mapper.spec.ts, 3 tests, passing
- last change: 2 months ago, reviewed

**Proposed label:** none

**Your label:** ✅ none


## 10. hx-untested-cancel

*new service method with branching and no test*

**Target** `src/app/orders/orders.service.ts#cancel` · **Instrument** coverage · **Trail** `orders.service.ts  ✅`

```ts
cancel(order: Order): Observable<void> {
  if (order.status === 'SHIPPED') return throwError(() => new Error('already shipped'));
  if (order.paidAt && Date.now() - order.paidAt.getTime() > 14 * 864e5) {
    return this.http.post<void>(`/api/orders/${order.id}/return`, {});
  }
  return this.http.delete<void>(`/api/orders/${order.id}`);
}
```

**Facts (trusted)**

- files: orders.service.ts, orders.service.spec.ts
- tests: orders.service.spec.ts covers list() and save(); no test calls cancel()
- coverage: cancel() 0% of 3 branches
- last change: cancel() added yesterday

**Proposed label:** 🧪

**Your label:** ✅ 🧪


## 11. hx-missing-guard

*route table imports a guard file that does not exist*

**Target** `src/app/app.routes.ts` · **Instrument** compiler · **Trail** `app.routes.ts  ❌`

```ts
import { Routes } from '@angular/router';
import { adminGuard } from './auth/admin.guard';

export const routes: Routes = [
  { path: 'orders', loadComponent: () => import('./orders/orders.page').then(m => m.OrdersPage) },
  { path: 'admin', canActivate: [adminGuard], loadComponent: () => import('./admin/admin.page').then(m => m.AdminPage) },
];
```

**Facts (trusted)**

- files in src/app/auth: auth.service.ts, auth.interceptor.ts (no admin.guard.ts)
- build: error TS2307: Cannot find module './auth/admin.guard'

**Proposed label:** 🌱

**Your label:** ✅ 🌱


## 12. hx-fresh-refund-rounding

*yesterday's change to refund rounding in a payment path*

**Target** `src/app/checkout/refund.ts#refundAmount` · **Instrument** git recency + path · **Trail** `refund.ts  ✅`

```ts
export function refundAmount(paidCents: number, usedDays: number, totalDays: number): number {
  const ratio = 1 - usedDays / totalDays;
  return Math.ceil(paidCents * ratio);   // was Math.floor until yesterday
}
```

**Facts (trusted)**

- files: refund.ts, refund.spec.ts
- tests: refund.spec.ts, 2 tests, passing (updated in the same commit)
- last change: yesterday, by an ageant, not reviewed; path src/app/checkout is marked sensitive

**Proposed label:** 👀 🐛

**Your label:** ✅ 👀 🙋


## 13. hx-state-copy-in-component

*component keeps and mutates its own copy of store state, against a grown plot*

**Target** `src/app/orders/orders.page.ts#archive` · **Instrument** plot proof · **Trail** `orders.page.ts  ✅`

**Garden**

```
🍄 Order state lives in the NgRx store. Components dispatch actions and never change a copy of the state.
   Contract 📌 orders.actions.ts  archiveOrder({ id })
```

```ts
export class OrdersPage implements OnInit {
  orders: Order[] = [];
  constructor(private store: Store) {}
  ngOnInit() { this.store.select(selectOrders).subscribe(o => this.orders = o); }
  archive(id: string) {
    this.orders = this.orders.filter(o => o.id !== id);
  }
}
```

**Facts (trusted)**

- files: orders.page.ts, .html, .spec.ts, orders.actions.ts, orders.reducer.ts
- tests: orders.page.spec.ts, 2 tests, passing
- orders.actions.ts defines archiveOrder({ id })

**Zone `orders`** (shown to probes only with `-zone`)

`src/app/orders/orders.reducer.ts`

```ts
on(archiveOrder, (s, { id }) => ({ ...s, orders: s.orders.map(o => o.id === id ? { ...o, archived: true } : o) }))
```

`src/app/orders/order-detail.page.ts`

```ts
archive(): void { this.store.dispatch(archiveOrder({ id: this.order.id })); }
```

**Proposed label:** 🧩

**Your label:** ✅ 🧩


## 14. hx-consent-wording

*new wording of the cookie consent banner*

**Target** `src/assets/i18n/fr.json` · **Instrument** path rule · **Trail** `fr.json  (no signals)`

```ts
{
  "consent.title": "Vos données, vos choix",
  "consent.body": "Nous utilisons des cookies pour améliorer votre expérience. En continuant, vous acceptez leur utilisation.",
  "consent.accept": "J'ai compris"
}
```

**Facts (trusted)**

- files: fr.json, en.json
- last change: today, consent.body and consent.accept rewritten
- no test covers translation text

**Proposed label:** 🙋

**Your label:** ✅ 🙋 👀


## 15. hx-query-builder-undocumented

*exported helper with many options, used widely, no documentation*

**Target** `src/app/shared/query.ts#buildQuery` · **Instrument** lint + fan-in · **Trail** `query.ts  ✅`

```ts
export function buildQuery(o: { q?: string; p?: number; s?: number; o?: string; d?: boolean; f?: Record<string, string[]> }): string {
  const parts: string[] = [];
  if (o.q) parts.push(`q=${encodeURIComponent(o.q)}`);
  if (o.p !== undefined) parts.push(`page=${o.p}`);
  if (o.s !== undefined) parts.push(`size=${o.s}`);
  if (o.o) parts.push(`sort=${o.o}${o.d ? ',desc' : ''}`);
  for (const [k, v] of Object.entries(o.f ?? {})) parts.push(`${k}=${v.join('|')}`);
  return parts.join('&');
}
```

**Facts (trusted)**

- files: query.ts, query.spec.ts
- tests: query.spec.ts, 4 tests, passing
- callers: 14 files
- lint: jsdoc/require-jsdoc reports query.ts line 1

**Proposed label:** 📝

**Your label:** ✅ 📝


## 16. hx-twin-mappers

*two near-identical mapping functions in one file*

**Target** `src/app/invoices/invoice-export.ts#toCsvRow` · **Instrument** duplication detector · **Trail** `invoice-export.ts  ✅`

```ts
export function toCsvRow(i: Invoice): string {
  return [i.number, i.client.name, i.total.toFixed(2), i.issuedAt.toISOString().slice(0, 10)].join(';');
}

export function toCsvRowForAccounting(i: Invoice): string {
  return [i.number, i.client.name, i.total.toFixed(2), i.issuedAt.toISOString().slice(0, 10), i.vatRate].join(';');
}
```

**Facts (trusted)**

- files: invoice-export.ts, invoice-export.spec.ts
- tests: invoice-export.spec.ts, 2 tests, passing
- duplication: 2 blocks, 1 line differs

**Zone `invoices`** (shown to probes only with `-zone`)

`src/app/invoices/invoice.mapper.ts`

```ts
export function toInvoice(d: InvoiceDto): Invoice { /* ... */ }
```

`src/app/invoices/accounting-export.ts`

```ts
rows = invoices.map(toCsvRowForAccounting);
```

**Proposed label:** 🧹

**Your label:** ✅ 🧹

