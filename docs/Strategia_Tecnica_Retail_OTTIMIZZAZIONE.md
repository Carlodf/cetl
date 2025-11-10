# Strategia Tecnica – Piattaforma di Ottimizzazione Prezzi, Scorte e Distribuzione (Multi‑store)

## 1) Problem Statement chiaro
L’azienda gestisce più punti vendita e desidera:
- **Prezzi al pubblico ottimali** per massimizzare marginalità o quota di mercato in funzione del contesto competitivo.
- **Pianificazione acquisti e scorte** per minimizzare rotture di stock e giacenze inutilizzate, migliorando il capitale circolante.
- **Allocazione verso i punti vendita** in base alla domanda attesa e ai vincoli logistici.
- **Intelligence competitiva** per identificare manovre dei concorrenti (es. prezzi anomali finalizzati a svuotare le nostre scorte) e reagire tempestivamente.
- **Governance e compliance** su dati sensibili (contratti, costi, API interne).

### Obiettivi misurabili (esempi)
- +2–5pp **margine lordo** su categorie chiave entro 90 gg.
- −20–30% **OOS rate** (stock-out) su top‑100 SKU.
- −10–15% **days of inventory** medio a parità di vendite.
- **Lead time decisionale** < 24h per aggiornamenti di prezzo e riordini.
- **SLA dati**: pipeline D+0 entro le 06:00 locali.

---

## 2) Analisi dei dati forniti
**Disponibili:**
- Schede prodotto (SKU, attributi, categorie, varianti).
- **Costo di acquisto** per SKU, specifico dell’attività (attenzione alla riservatezza).
- **Giacenze di magazzino** (magazzino centrale + store).
- **Stream prezzi concorrenti** (richiede normalizzazione e SKU matching).

**Mancanti ma raccomandati:**
- Storico vendite per SKU×store×giorno (almeno 12–18 mesi).
- Calendario promozionale e scontistiche; eventuali regole MAP/MRP.
- Lead time fornitori, lotti minimi (MOQ), vincoli budget.
- Costi logistici (trasferimento tra magazzino e punti vendita).
- Eventi esterni (meteo/festività) ove rilevanti.

**Quality check e normalizzazione (high level):**
- **MDM**: chiavi uniche per SKU, store, fornitore.
- **SKU matching** verso la concorrenza: regole + embedding testuali/immagine con validazione assistita.
- Deduplica EAN, unificazione UoM (unità di misura), gestione varianti/pack.
- Storicizzazione **slowly changing** dei listini costi/fornitori.
- Validazione giacenze (coerenza con movimenti, stock negativo, lag di aggiornamento).

---

## 3) Approccio a fasi (30/60/90)

### Giorno 0–30: **Fondazioni e baseline**
- **Data pipeline minima** (ingest + storage) per i dataset disponibili; definizione MDM.
- **Baseline forecasting**: Prophet/SARIMA per SKU non intermittenti; Croston/SBA per intermittenti.
- **Dashboard iniziali** in BI (metrica canonica: vendite, margine, DOS, OOS).
- **SKU matching v1** per prezzi competitor (precisione target ≥85% su top‑SKU).
- **Security & compliance**: classificazione dati (sensibili vs non sensibili), ruoli e accessi.
**Deliverable:** cruscotti, report qualità dati, baseline modelli, runbook ETL, decision log.

### Giorno 31–60: **Modelli globali e ottimizzazione**
- **Forecast globale** con LightGBM/XGBoost + feature esterne (prezzi propri/competitor, calendario, promo, stock).
- **Stima elasticità** (panel log‑log o ML con interpretabilità SHAP).
- **Solver inventory & allocation** (OR‑Tools): riordino, safety stock, ripartizione ai negozi.
- **Rules & alerting** per anomalie concorrenza (downpricing coordinato, outlier).
- **PoC pricing dinamico** su 1–2 categorie (shadow mode → A/B controllato).
**Deliverable:** API interne v1 (forecast/price/alloc), playbook pricing, alerting competitivo.

### Giorno 61–90: **Industrializzazione e governance**
- **Hardening pipeline** (test, osservabilità, data quality SLO).
- **Knowledge Layer (opzionale ma consigliato)**: ontologia prodotto/fornitore/store, vincoli (MAP, MOQ, lead time), sostituti/complementi.
- **MLOps**: retraining programmato, backtest rolling, champion/challenger.
- **Scale‑out** categorie e punti vendita; **what‑if simulator** per prezzo e scorte.
**Deliverable:** rilascio in produzione, runbook incident, KPI di impatto, roadmap 6 mesi.

---

## 4) Architettura di alto livello

### 4.1 Opzione Cloud (es. AWS/Azure/GCP – vendor neutral)
- **Ingestion & Stream**: connettori batch + stream (es. Kafka managed).
- **Storage**: Data Lake (object storage) + Warehouse (SQL columnar).
- **Compute**: jobs ETL/ELT (dbt + orchestratore), ML (serverless/containers), OR‑Tools.
- **Feature Store** (es. Feast) e **Model Registry**.
- **Serving**: API Gateway + Functions/Containers per suggerimenti in tempo quasi reale.
- **BI & Alerting**: dashboard, metriche SLO, notifiche.
- **Security**: IAM, KMS/HSM, VPC, secret manager.
**Pro:** time‑to‑value rapido, scalabilità elastica, servizi gestiti.  
**Contro:** dati sensibili in cloud (mitigabile con cifratura + private link), lock‑in parziale.

### 4.2 Opzione On‑prem
- **Ingestion**: connettori su rete interna e batch scheduler.
- **Storage**: data lake su NAS/obj store on‑prem + warehouse (es. PostgreSQL/ClickHouse).
- **Compute**: cluster Kubernetes bare‑metal o VM per ETL/ML/solver.
- **Serving & BI**: gateway interno, SSO aziendale, SIEM locale.
**Pro:** controllo totale dei dati sensibili, integrazione con legacy.  
**Contro:** CAPEX alto, scalabilità più rigida, oneri operativi (patching, HA/DR).

### 4.3 Opzione Ibrida (consigliata)
- **Dati sensibili e API interne on‑prem** (costi, contratti, identity).
- **Dati non sensibili e compute intensivo in cloud** (feature derivata, training, simulazioni).
- **Connettività**: VPN/Direct Connect/ExpressRoute; **cifratura end‑to‑end**, tokenizzazione dei dati sensibili che transitano.
- **Pattern**: *train in cloud, serve on‑prem* per endpoint che usano dati sensibili; viceversa per casi low‑risk.
**Pro:** equilibrio tra compliance e scalabilità, ottimizzazione dei costi.  
**Contro:** complessità di rete e governance; necessaria chiara **data classification** e **catalog**.

---

## 5) Costi (stime high level, IVA/overhead esclusi)
> Le cifre variano per volumi (SKU×store), frequenze di aggiornamento e scelte vendor. Range ordinati per grandezza.

### 5.1 Setup iniziale (una tantum)
- **Analisi dati, MDM, modellazione baseline**: €12k–€25k
- **Pipeline ETL/ELT + BI base**: €15k–€30k
- **SKU matching v1 (regole+embedding) + validazione**: €8k–€18k
- **PoC modelli globali + solver inventory/allocation**: €20k–€40k
- **Security & IAM, hardening minimo**: €5k–€12k  
**Totale setup**: **€60k–€125k**

### 5.2 OPEX mensile (cloud, volumi medi)
- **Compute & storage**: €1.5k–€4k/mese
- **BI & log/monitoring**: €300–€800/mese
- **Manutenzione/MLOps** (1–2 gg/settimana): €3k–€8k/mese  
**Totale OPEX cloud**: **€4.8k–€12.8k/mese**

### 5.3 OPEX mensile (on‑prem)
- **HW/ammortamento + energia**: €2k–€6k/mese (variabile)
- **Ops & patching**: €2k–€5k/mese
- **Supporto applicativo/MLOps**: €3k–€8k/mese  
**Totale OPEX on‑prem**: **€7k–€19k/mese** (al netto del CAPEX iniziale).

### 5.4 Costi aggiuntivi opzionali
- **Knowledge Graph & semantic layer**: €10k–€25k setup; €1k–€3k/mese.
- **Simulatore what‑if & A/B**: €8k–€20k setup.
- **Integrazione POS/ERP avanzata**: €5k–€20k.

---

## 6) Rischi & mitigazioni (estratto)
- **Qualità/mancanza storico vendite** → baseline conservative, enrichment progressivo; focus top‑SKU.
- **SKU matching incompleto** → pipeline assistita (human‑in‑the‑loop), soglia confidenza, fallback categoria.
- **Vincoli contrattuali (MAP/MRP)** → centralizzazione in knowledge layer; validazione in solver.
- **Cambi organizzativi** → governance RACI chiara, runbook e formazione.
- **Lock‑in tecnologico** → astrazione via dbt/Feast/containers, IaC, portabilità modelli.

---

## 7) KPI & governance
- **Accuracy previsioni**: MASE/sMAPE + Pinball loss (P50/P90).
- **Service level**: fill rate, OOS rate, days of supply.
- **Economici**: GM%, GMROI, valore stock, riduzione markdown.
- **Operativi**: latenze ETL/API, tasso alert risolti, tempi ricalcolo.
- **Sicurezza**: audit accessi, encryption at rest/in transit, conformità policy.

---

## 8) Raccomandazione
Avviare con approccio **30/60/90** e **architettura ibrida**: dati sensibili e serving critico **on‑prem**, training e calcoli pesanti **in cloud**. Consolidare i fondamentali (MDM, qualità dati, baseline) nei primi 30 gg, poi modelli globali e solver entro 60 gg, quindi hardening e scale‑out entro 90 gg. Aspettativa: impatto su margine e disponibilità stock già dalle prime due categorie pilota.
