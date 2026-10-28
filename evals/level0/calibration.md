# Level 0 calibration sheet

No model got these 7 fixtures right in any run (qwen3:4b and phi4-mini, three menus each).
Either the models cannot do it, or the fixture is unfair: ambiguous code, a hint missing, or a label a careful human would not give.
There is no strong model to settle it, so a human does, once. This is the colony's founding phase in miniature.

For each case, tick one decision and add a line on why:

- **Keep**: the label is right and a careful reader would give it from what the terrain shows.
- **Correct**: the label is wrong or too narrow. Write the accept list you would use.
- **Drop**: the case cannot be judged from what is shown. It leaves the set.

Decisions are applied to `fixtures.json` and recorded in the findings doc. They are made before the receptor run is read, so that the run cannot influence them.



## Decisions (human calibration, 2026-10-26)

Made before any receptor result was read.

| # | Fixture | Decision |
|---|---|---|
| 1 | `copy-paste-methods` | keep, 🧹 only |
| 2 | `iban-check` | keep, 📝 only |
| 3 | `toast-in-service` | correct → accept 🧩 🐛 |
| 4 | `any-lookup` | correct → accept ❓ 📝 |
| 5 | `brand-colour-change` | correct → rewritten, accept 🧩 🙋 |
| 6 | `message-tone` | moved to `evals/beyond-scout/` |
| 7 | `legal-mention` | correct → accept none 🙋 |

**G1, applies to all fixtures.** Ageants were told about tests only through code comments, and a comment can lie. Not trusting it is good behaviour, so the fixtures, not the ageants, were at fault. Everything the Ecosystem would compute (files, tests and their status, callers, git history, failing checks) now reaches the model in a separate, trusted FACTS block after the terrain. The information is unchanged; only the channel moved.

**Hypothesis H1, to be tested.** Ageants look for issues and fall back on "add a test" or "add documentation" when they find none: the genome's wording makes them over-cautious. Tested by a third arm, scout `scout-l0-c`, where `none` is one of the signals, chosen like any other.

---

## 1. `copy-paste-methods` — two near-identical methods, tested

**Target** `src/app/credit/export.service.ts#exportCsv` · **label** 🧹 · **instrument** duplication detector
· **trail** `export.service.ts  ✅`

```ts
// folder src/app/credit/: export.service.ts, export.service.spec.ts (passing)
export class ExportService {
  exportCsv(rows: Credit[]): string {
    const header = 'id;amount;status';
    const lines = rows.map(r => [r.id, r.amount, r.status].join(';'));
    return [header, ...lines].join('\n');
  }
  exportTsv(rows: Credit[]): string {
    const header = 'id\tamount\tstatus';
    const lines = rows.map(r => [r.id, r.amount, r.status].join('\t'));
    return [header, ...lines].join('\n');
  }
}
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | 🧪 | The exportCsv method could benefit from a test to ensure it handles various edge cases correctly. |
| phi4-mini | emoji | 🧪 | The CSV export function could benefit from a test to ensure its correctness and reliability. |
| phi4-mini | names | 🧪 | The exportCsv method could benefit from a test to ensure its correctness and handle edge cases. |
| qwen3:4b | both | ❓ | The code for exportCsv is simple and well-formed, converting Credit[] to CSV without issues. No obvious missing dependencies, bugs, or documentation needs. |
| qwen3:4b | emoji | 🌱 | The exportCsv method converts Credit[] to CSV string with proper headers and line breaks. It appears to work correctly for the given input format. |
| qwen3:4b | names | 🧪 | The code exports CSV and TSV strings from Credit[] rows without issues, but it lacks tests for the export methods. |

**Decision:** keep, 🧹 only

**Why:** A perfect ageant reads the target in its context: the near-identical neighbour method makes "works but needs refactoring" the right call. Not trusting the comment about passing tests is good behaviour (see G1).


---

## 2. `iban-check` — domain algorithm with no explanation, tested

**Target** `src/lib/iban.ts#validIban` · **label** 📝 · **instrument** lint + fan-in
· **trail** `iban.ts  ✅`

```ts
// folder src/lib/: iban.ts, iban.spec.ts (passing). Used by the payment forms.
export function validIban(iban: string): boolean {
  const s = iban.replace(/\s/g, '').toUpperCase();
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(s)) return false;
  const r = (s.slice(4) + s.slice(0, 4)).replace(/[A-Z]/g, c => String(c.charCodeAt(0) - 55));
  return BigInt(r) % 97n === 1n;
}
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | 🧪 | The code lacks comprehensive test coverage for various edge cases and potential invalid inputs. |
| phi4-mini | emoji | 🧪 | This function lacks comprehensive test coverage for various edge cases and invalid inputs. |
| phi4-mini | names | 🧪 | The code lacks comprehensive test coverage for various edge cases and invalid inputs. |
| qwen3:4b | both | 🐛 | The function validIban checks if an IBAN is valid using a regex and a modulus check, but it's unclear if it handles edge cases like empty strings or non-IBAN… |
| qwen3:4b | emoji | 🌱 | The function validIban checks if an IBAN is valid using a regex and a modulus check. It seems to be working correctly as per the test file (iban.spec.ts) bei… |
| qwen3:4b | names | ❓ | The function validIban checks if an IBAN is valid using a regex and a modulus check, but it's unclear if it handles edge cases like empty strings or non-IBAN… |

**Decision:** keep, 📝 only

**Why:** Heavy reliance on domain logic nobody reading the code knows by default; maintenance would be a nightmare without retro-documentation. ❓ was considered and rejected: the name makes the purpose clear, the why of the algorithm is what is missing.


---

## 3. `toast-in-service` — service shows a toast itself, against a grown plot

**Target** `src/app/credit/credit.service.ts#remove` · **label** 🧩 · **instrument** plot proof
· **trail** `credit.service.ts  ✅`

**Garden**

> 🍄 HTTP errors are mapped once, in ErrorMapper. Services never show messages to the user.

```ts
// folder src/app/credit/: credit.service.ts, credit.service.spec.ts (passing)
remove(id: string): Observable<void> {
  return this.http.delete<void>(`/api/credits/${id}`).pipe(
    catchError(err => {
      this.toast.error('Suppression impossible');
      return throwError(() => err);
    }),
  );
}
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | 🐛 | The remove method should handle HTTP errors gracefully instead of suppressing them with a toast error |
| phi4-mini | emoji | 🧪 | The remove method lacks a test for the successful HTTP delete response |
| phi4-mini | names | 🧪 | The remove method lacks a test case for error handling when the HTTP request fails |
| qwen3:4b | both | 🐛 | The remove method uses a toast.error to show a user message on error, but according to the garden, HTTP errors should be mapped once in ErrorMapper and servi… |
| qwen3:4b | emoji | 🐛 | The remove method uses a toast error message without checking if the error is a 404 or other specific error, which might lead to unnecessary user feedback fo… |
| qwen3:4b | names | 🐛 | The remove method handles HTTP DELETE requests and shows a toast error if there's an issue, but it doesn't handle HTTP errors properly as per the garden's ac… |

**Decision:** correct → accept 🧩 🐛

**Why:** Flagging a bug is also correct when code breaks an accepted decision; both route the target to the right place.


---

## 4. `any-lookup` — untyped lookup through a global, purpose unclear

**Target** `src/app/legacy/cfg.ts#doIt2` · **label** ❓ · **instrument** none

```ts
// folder src/app/legacy/: cfg.ts (no spec). Called from 3 places, all named doIt2(...)
export function doIt2(a: any, b: any) {
  return a?.k?.[b] ?? (window as any)['__cfg']?.[b] ?? a?.['_' + b];
}
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | 📝 | The function lacks documentation explaining its purpose and usage. |
| phi4-mini | emoji | 📝 | The function lacks documentation explaining its purpose and usage. |
| phi4-mini | names | 🐛 | The function lacks proper error handling and type safety, which could lead to runtime errors if 'a' or 'b' are not structured as expected. |
| qwen3:4b | both | 📝 | The function `doIt2` is a legacy function that attempts to access configuration values in multiple ways but lacks documentation and tests. |
| qwen3:4b | emoji | 📝 | The function doIt2 is a legacy function that tries to access configuration values in multiple ways but lacks clear documentation and is used in multiple plac… |
| qwen3:4b | names | 📝 | The function doIt2 is a legacy function that tries to access configuration values in multiple ways but lacks clear documentation and may be error-prone due t… |

**Decision:** correct → accept ❓ 📝

**Why:** "Lacks documentation" and "cryptic" lead to the same work for code nobody can explain, consistent with `cryptic` and `bit-twiddling`.


---

## 5. `brand-colour-change` — visual change only a human can judge

**Target** `src/app/credit/credit-actions.component.scss` · **label** 🙋 · **instrument** path rule
· **trail** `credit-actions.component.scss  ✅`

```scss
/* folder src/app/credit/. Changed one tick ago: primary button colour and size. */
.primary {
  background: #ff3b7f;
  border-radius: 24px;
  font-size: 18px;
  text-transform: uppercase;
}
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | none | The SCSS file appears to be correctly formatted and passed. |
| phi4-mini | emoji | 🧪 | The SCSS file is missing a test for the primary button style changes. |
| phi4-mini | names | 🧪 | The SCSS file lacks a test case for the primary button color and size changes. |
| qwen3:4b | both | ❓ | The CSS file defines a .primary class with specific styling for a button, but there's no indication of missing files, tests, bugs, or documentation needs. Th… |
| qwen3:4b | emoji | none | The CSS class .primary defines a styled button with specific colors and sizes, but there's no indication of what it needs beyond existing styles. The code ap… |
| qwen3:4b | names | ❓ | The CSS file defines a primary button style with specific colors and sizes, but there's no indication of missing files, tests, bugs, or documentation needs. … |

**Decision:** correct → rewritten, accept 🧩 🙋

**Why:** Cryptic even to a human: nothing showed what was wrong. Rewritten with a grown plot on design-system tokens (proof: stylelint), the neighbour file using `var(--…)`, and this file using raw values. Worth keeping: ageants could shine on such "invisible" issues.


---

## 6. `message-tone` — user-facing wording changed, tone is a product choice

**Target** `src/app/credit/messages.fr.ts` · **label** 🙋 · **instrument** path rule
· **trail** `messages.fr.ts  ✅`

```ts
// folder src/app/credit/. Changed one tick ago.
export const messages = {
  amountRequired: 'Hé ! Il faut un montant, là.',
  noResult: 'Rien trouvé, désolé 🙃',
  saved: 'Enregistré !!!',
};
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | none | The code is a simple object with no complex logic or dependencies that require testing. |
| phi4-mini | emoji | 🧪 | The code lacks tests for its functionality |
| phi4-mini | names | 📝 | The code lacks documentation for the strings provided |
| qwen3:4b | both | ❓ | This file exports French messages for credit app, seems to be a simple translation file with no obvious issues. |
| qwen3:4b | emoji | none | This file exports French messages for credit app, seems to be a simple translation file with no issues. |
| qwen3:4b | names | ❓ | This file exports French messages for credit app, seems well-defined and no obvious issues. |

**Decision:** moved to `evals/beyond-scout/`

**Why:** Interesting, but beyond the intelligence of a classical small ageant. Kept for other ageant types (patroller or gardener with a honey cell defining the product's tone).


---

## 7. `legal-mention` — regulated wording on a credit offer

**Target** `src/app/credit/offer-footer.component.html` · **label** 🙋 · **instrument** path rule
· **trail** `offer-footer.component.html  ✅`

```html
<!-- folder src/app/credit/. Changed one tick ago: the legal mention below. -->
<p class="legal">
  Un crédit vous engage et doit être remboursé. Taux annuel effectif global fixe : 3,9 %.
  Offre valable sous réserve d'acceptation.
</p>
```

| Model | Menu | Said | Its note |
|---|---|---|---|
| phi4-mini | both | 🧪 | The HTML component lacks any JavaScript or TypeScript code to test its functionality. |
| phi4-mini | emoji | 🧪 | The legal mention is outdated and needs updating to reflect current laws and rates |
| phi4-mini | names | 📝 | The legal mention is outdated and needs updating to reflect current laws and rates. |
| qwen3:4b | both | none | The HTML file contains a legal notice text that is static and well-formed, with no obvious issues or missing elements. It appears to be correctly formatted a… |
| qwen3:4b | emoji | none | The HTML file contains a legal notice text that is static and well-formed, no obvious issues or missing elements detected. |
| qwen3:4b | names | none | The HTML file contains a legal notice text that is static and well-formatted, no obvious issues or missing elements detected. |

**Decision:** correct → accept none 🙋

**Why:** Nothing is visibly wrong, so "nothing to do" is a right answer. Who approves legal wording is a rule outside the code (a path rule in the settings), not a model's judgment.

