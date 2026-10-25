---
name: allod-ux-engine
description: >-
  Senior-designer reasoning engine and UI/UX design system enforcer for Allod Panel.
  Use whenever creating, modifying, auditing, or restyling frontend code (HTML, CSS, JS)
  in cmd/allod-panel/web/*, or when the user invokes /ux-design, /restyle, or /ux-audit.
---

# Allod UX Engine

The **Allod UX Engine** enforces senior-designer reasoning before generating frontend code for the Allod sovereign node panel (`cmd/allod-panel/web/*`). It prevents generic, database-shaped AI interfaces and guarantees a cohesive, high-end, sovereign product feel (styled after Linear, Vercel, and Shadcn Dark tokens).

---

## The 9 Senior-Designer Reasoning Skills

Whenever you design, modify, or audit any user interface in Allod, you MUST execute these 9 skills:

### 1. Intent Discovery (Scopo & Intento)
Before writing HTML or CSS, answer:
* **Chi usa questa schermata?** (es. l'amministratore del NAS, un familiare con accesso ospite, o un demone/bot).
* **Qual è l'obiettivo primario (Job-to-be-Done)?** (es. salvare una chiave API, verificare lo stato del disco).
* **Qual è l'errore peggiore e irreversibile possibile?** (es. formattare il disco sbagliato, eliminare un nodo dal Ring WireGuard, esporre un segreto). Quella specifica azione DEVE essere protetta e isolata.

### 2. Information Hierarchy (Gerarchia dell'Informazione)
* Un'unica azione primaria dominante per schermata/modale (pulsante pieno `var(--primary)` o `var(--success)`).
* Azioni secondarie in stile ghost o outline con bordo sottile.
* Separazione netta delle azioni distruttive (in basso a sinistra o dentro menu di overflow).
* Raggruppamento logico con spaziature coerenti a griglia 4px (`--space-1` .. `--space-8`).

### 3. State Completeness (I 6 Stati Obbligatori)
Nessun componente o card è pronto per il rilascio senza aver previsto tutti i 6 stati:
1. **Idle**: visualizzazione standard con dati popolati.
2. **Loading**: stato di caricamento inline (<200ms) o skeleton loader (>500ms).
3. **Empty**: mai limitarsi a "Nessun dato". Fornire icona tematica, spiegazione rassicurante e CTA primaria per iniziare.
4. **Error**: banner o box con sfondo tenue `rgba(239, 68, 68, 0.1)`, diagnosi umana e tasto "Riprova".
5. **Partial / Stale**: avviso discreto quando i dati non sono sincronizzati o la mesh è offline.
6. **Success**: notifica visiva istantanea (usare `showToast(msg, 'success')`).

### 4. Form UX (Esperienza dei Moduli)
* Etichette `<label>` sempre visibili sopra l'input (`font-size: 12px; font-weight: 600;`).
* Placeholder usati solo come esempio di formato (es. `es. nbp_live_xxxxxxxx`), non come sostituto dell'etichetta.
* Testo d'aiuto `<small>` posizionato immediatamente sotto il campo per guidare l'utente.
* **Regola "Anti-Disabled Button"**: non disabilitare mai i pulsanti in modo silenzioso senza spiegare il motivo. Piuttosto lascia il pulsante attivo e, se cliccato, evidenzia i campi mancanti con un messaggio chiaro.

### 5. Feedback & Affordance
* Ogni interazione (click, copia, invio) deve produrre feedback visivo entro 100 millisecondi.
* Utilizzare `showToast(msg, type)` per notifiche fluttuanti rapide.
* Stati `:hover`, `:focus-visible` e `:active` chiaramente definiti per ogni bottone e riga interattiva.

### 6. UX Auditor
Analizza qualsiasi schermata esistente e restituisci:
* **Severità Critica**: blocchi, rischi di perdita dati, assenza di feedback, bottoni muti.
* **Severità Media**: problemi di gerarchia visiva, spaziature incoerenti, testo illeggibile.
* **Severità Bassa**: rifiniture estetiche, micro-animazioni, allineamenti.
* Proponi sempre il codice corretto direttamente applicabile.

### 7. Design System Enforcer
Fai sempre riferimento al file [`DESIGN.md`](DESIGN.md) alla radice del repository:
* Utilizza esclusivamente i token CSS definiti in `style.css` (`--background`, `--card`, `--primary`, `--border-subtle`, `--radius-*`, `--font-size-*`).
* Zero colori esadecimali arbitrari inline.
* Zero dipendenze esterne (Google Fonts, CDN JS): Allod funziona in totale isolamento LAN/Air-gapped.

### 8. Visual Character (Finiture Tecniche e Sovrane)
* Stile dark pulito, moderno e rifinito (stile Linear / Shadcn).
* Bordi sottili semitrasparenti (`border-subtle: rgba(255, 255, 255, 0.06)`).
* Card selezionabili arricchite con CSS `:has(input:checked)`.
* Evitare l'aspetto piatto o rozzo tipico dei template HTML basilari.

### 9. Responsive Engine (< 640px)
* Modali adattabili a tutta altezza su smartphone.
* Touch target minimi di `40px` - `44px`.
* Tabelle dense trasformate in card o con scroll orizzontale esplicito.

---

## Comandi Operativi Slash

L'utente (o l'agente) può invocare direttamente queste modalità operative:

### 1. `/ux-design [funzionalità]`
Pianifica da zero una nuova interfaccia.
* Esegui prima l'**Intent Discovery** (3 domande chiave su utente, obiettivo e peggior errore).
* Definisci la gerarchia delle azioni e la mappa dei 6 stati.
* Presenta le specifiche e il wireframe prima di scrivere il codice definitivo.

### 2. `/restyle [componente o modale]`
Rifinisce graficamente un componente esistente applicando [`DESIGN.md`](DESIGN.md).
* Ispeziona i 4 assi (Tipografia, Colore, Spazio, Finiture).
* Sostituisce stili inline e stili generici con i token CSS ufficiali.
* Mostra un confronto prima/dopo lasciando inalterata la logica di business.

### 3. `/ux-audit [sezione]`
Conduce una revisione esperta della UI secondo i 9 principi e restituisce il report ordinato per severità con i fix pronti.
