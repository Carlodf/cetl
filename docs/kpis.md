## Retail basics in 15 minutes

**Mental model**

- **P&L ladder:** Vendite nette → **COGS** → **GM%** → Opex (personale, affitti, logistica) → **Store/Four-wall EBITDA**.
- **Motore cassa:** l’inventario immobilizza capitale; il gioco è bilanciare **margine × volume × inventario**.
- **Trade-off chiave:** (1) **Prezzo vs volume** (elasticità), (2) **Service level vs stock** (rotture vs immobilizzo).

---

## KPI essenziali (cheat-sheet)

| KPI                               | Formula (semplificata)                                                                                        | Perché conta                           | Note pratiche                                     |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------- | -------------------------------------- | ------------------------------------------------- |
| **Vendite nette**                 | vendite lorde − resi/sconti                                                                                   | Misura la domanda effettiva            | Segmenta per categoria/store                      |
| **COGS (costo del venduto)**      | somma costi diretti venduti                                                                                   | Base per margine                       | Includi acquisto + inbound diretti                |
| **GM% (margine lordo)**           | (Vendite − COGS) / Vendite                                                                                    | Quanta value catturi per € venduto     | Evita confronto tra periodi con mix molto diverso |
| **GMROI**                         | Margine lordo / Valore medio stock                                                                            | Redditività del capitale in inventario | Se ↑ mentre **fill rate** resta alto ⇒ ottimo     |
| **Inventory Turns**               | COGS / Inventario medio (annuo)                                                                               | Velocità con cui giri lo stock         | ≈ **365 / DOI(value)**                            |
| **DOI / DSI (Days of Inventory)** | **Ops (unit):** Stock unit / Vendite medie giornaliere \| **Contabile (value):** (Inv. medio / COGS) × Giorni | Copertura giorni; driver di cash       | Usa entrambe le viste (unit & value)              |
| **Fill rate / Service level**     | righe (o pezzi) evase / richieste                                                                             | Protegge ricavi ed esperienza cliente  | Obiettivo tipico ≥ 95% su SKU chiave              |
| **OOS rate**                      | % tempo o righe non disponibili                                                                               | Vendite perse                          | Correggi le vendite medie per giorni OOS          |
| **Sell-through**                  | Venduto / (Venduto + Giacenza)                                                                                | Assorbimento del ricevuto              | Utile per promo, lanci, fine stagione             |
| **Markdown %**                    | sconti / vendite lorde                                                                                        | Erosione margine                       | Traccia motivi (stagionalità, obsolescenza)       |
| **Price Index**                   | prezzo medio vs mercato                                                                                       | Posizionamento competitivo             | Richiede matching SKU concorrenti                 |
| **LFL (Like-for-Like)**           | crescita vendite su negozi comparabili                                                                        | Isola performance dal network growth   | Normalizza per aperture/chiusure                  |

**Regola d’oro:** leggi sempre **GM% / GMROI / DOI / Fill rate** insieme: DOI↓ è buono solo se il **service level** non crolla.

---

## Definizioni operative (copiabili)

- **Vendite medie giornaliere (unit)** = somma vendite **escludendo giorni OOS** / #giorni utili
- **DOI (unit)** = `Stock_unità_oggi / Vendite_medie_giornaliere_unit`
- **DOI (value)** = `Valore_stock_oggi / COGS_medio_giornaliero`
- **GMROI** = `Margine lordo / Valore medio stock`
- **Inventory Turns (annuali)** ≈ `365 / DOI(value)`

**Target di esempio**

- **GM%**: +2–5 **pp** su categorie chiave (90 gg)
- **DOI**: **−10%** vs baseline **a parità di fill rate ≥ 95%**
- **OOS rate**: −20–30% su Top-100 SKU

---

## Pitfall comuni (e come evitarli)

1. **Ambiguità DOI/DSI** → dichiara sempre **quale formula** usi (ops vs contabile).
2. **Bias da stock-out** → escludi giorni OOS dal calcolo vendite medie.
3. **Mix/Prezzi cambiano** → calcola KPI **in unità e in valore**.
4. **Stagionalità/promozioni** → usa finestre rolling **28/56 gg** e confronti YoY comparabili.
5. **Medie che nascondono code** → affianca **percentili (P50/P90)** e “**aged inventory**” (% > 60/90 gg).

---

## Starter dashboard (minimo utile)

- **Revenue/Margin**: Vendite nette, **GM%**, **GMROI** (settimanale, per categoria & store)
- **Stock health**: **DOI (unit & €)**, **Turns**, **OOS rate**, **Aged inventory** (% stock > 60/90 gg)
- **Pricing**: **Price index** vs concorrenza, **Markdown %**, uplift promo
- **Service**: **Fill rate**, On-shelf availability

---

## 4-hour crash course (letture rapide)

> Obiettivo: farti capire **cosa** misurare e **perché**, senza libri interi. Cerca i titoli suggeriti; sono articoli/guide brevi e pratiche.

1. **Retail math primer (60–90 min)**
   - Introduzione a GM%, GMROI, Inventory Turns, Sell-through, Markdown%
   - Cerca: _“Retail math cheat sheet GMROI turns sell-through”_ (guide di vendor POS/retail sono ottime e brevi)

2. **Inventory KPIs & Days of Inventory (30–45 min)**
   - Differenza DOI (ops) vs DSI (contabile) + esempi numerici
   - Cerca: _“Days of inventory vs days sales in inventory explanation”_

3. **Service level & OOS (30 min)**
   - Fill rate, stock-out, availability; perché DOI va bilanciato col servizio
   - Cerca: _“fill rate vs service level retail explanation”_

4. **Pricing basics (45–60 min)**
   - Elasticità, price index, markdown management (concetti, non formula-heavy)
   - Cerca: _“retail pricing basics elasticity markdown introduction”_

5. **Category management overview (20–30 min)**
   - Ruolo categorie, KPI per categoria, spazio & mix
   - Cerca: _“category management 101 KPI overview”_

**Tip:** mentre leggi, annota 3 cose per KPI: **(a)** definizione “aziendale” che userai, **(b)** tabella/colonne di warehouse necessarie, **(c)** alert/decisione che abilita (es. DOI>P90 ⇒ redistribuzione/promo).

---

## Mappatura KPI → leve di sistema

- **GM% / GMROI** ⟶ pricing dinamico, riduzione markdown (miglior forecast)
- **DOI / Turns** ⟶ regole di riordino (quantili), **allocation** tra store, trasferimenti inter-store
- **OOS / Fill rate** ⟶ alert preventivi, ribilanci, lead time accurati
- **Price index** ⟶ integrazione prezzi concorrenti + elasticità
- **Sell-through / Markdown%** ⟶ fine stagione, pulizia stock, protezione margine

---
