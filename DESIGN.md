# Allod Design System & UI Specification (`DESIGN.md`)

> **Sovereign NAS & Distributed Backup Node**  
> Basato sui principi di design reasoning di Design Motion HQ & Shadcn Dark Tokens.  
> Questo documento è la singola fonte di verità per ogni schermata, modale e componente di Allod Panel (`cmd/allod-panel/web/*`).

---

## 1. Filosofia di Design ("Anti AI-Made UI")
Le interfacce di Allod non devono sembrare un dump grezzo di tabelle o dati del database ("database-shaped interfaces"). Devono trasmettere:
1. **Sovranità e Sicurezza**: atmosfera scura, tecnica, affidabile (stile Linear / Vercel / Hetzner Cloud).
2. **Zero Rumore Esterno**: nessun font da Google Fonts, nessun CDN esterno, zero telemetria. Tutti gli asset sono 100% self-hosted e locali.
3. **Densità Ragionata**: una dashboard per server NAS richiede densità informativa (tabelle scansionabili, badge chiari), ma con gerarchia impeccabile.

---

## 2. I 4 Assi Visivi (Design Tokens)

Tutti i token sono definiti come variabili CSS in `cmd/allod-panel/web/style.css`. È **vietato** utilizzare valori esadecimali o dimensioni arbitrarie inline.

### Asse 1: Tipografia (Type)
Font stack puramente di sistema (zero ritardi di caricamento, zero tracking):
* **Sans**: `system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`
* **Mono**: `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`

| Token | Dimensione | Utilizzo Principale |
| :--- | :--- | :--- |
| `--font-size-3xl` | `28px` | Titoli principali di pagina / Hero metrics |
| `--font-size-2xl` | `22px` | Titoli di sezione |
| `--font-size-xl` | `18px` | Intestazioni di card importanti |
| `--font-size-lg` | `16px` | Intestazioni card secondarie / Modali |
| `--font-size-base` | `14px` | Testo corpo standard / Input utente |
| `--font-size-sm` | `12.5px` | Didascalie, etichette campi form, tabelle dense |
| `--font-size-xs` | `11px` | Badge di stato, tag telemetria, timestamp |

### Asse 2: Colore (Color)
Palette ad alto contrasto dark-first conforme WCAG AA:
* **Background Principale**: `var(--background)` (`#090d16`)
* **Superfici Card**: `var(--card)` (`#0d131f`)
* **Superfici Secondarie/Hover**: `var(--secondary)` (`#162032`) / `var(--secondary-hover)` (`#1e293b`)
* **Testo Principale**: `var(--foreground)` (`#f8fafc`)
* **Testo Muted / Didascalie**: `var(--muted-foreground)` (`#94a3b8`)
* **Brand / Accento Primario**: `var(--primary)` (`#38bdf8` - Sky Blue), Hover: `#0ea5e9`, Glow: `rgba(56, 189, 248, 0.12)`
* **Bordi**:
  * `--border`: `#1e293b` (bordi divisori e separatori card)
  * `--border-subtle`: `rgba(255, 255, 255, 0.06)` (finiture interne eleganti)

#### Colori Semantici (Feedback):
* **Success**: `var(--success)` (`#10b981` - Emerald)
* **Warning**: `var(--warning)` (`#f59e0b` - Amber)
* **Destructive / Error**: `var(--destructive)` (`#ef4444` - Crimson)
* **Info**: `var(--info)` (`#38bdf8` - Sky)

### Asse 3: Spaziatura (Space)
Griglia rigorosa a multipli di 4px:
* `--space-1` (`4px`) - Spaziatura interna badge, padding compatti
* `--space-2` (`8px`) - Gap tra icone e testo, padding bottoni compatti
* `--space-3` (`12px`) - Padding form, gap tra elementi correlati
* `--space-4` (`16px`) - Padding standard card e sezioni modale
* `--space-5` (`20px`) - Separazione tra gruppi logici
* `--space-6` (`24px`) - Spaziatura tra card principali
* `--space-8` (`32px`) - Margini sezioni maggiori

### Asse 4: Finiture (Finish)
* **Raggi di curvatura**:
  * `--radius-sm`: `6px` (bottoni piccoli, badge, input)
  * `--radius-md`: `8px` (card secondarie, container form)
  * `--radius-lg`: `12px` (card principali, alert banner)
  * `--radius-xl`: `16px` (modali fluttuanti)
* **Profondità & Elevazione**:
  * Ombreggiatura modali: `box-shadow: 0 20px 40px rgba(0, 0, 0, 0.6)`
  * Effetto vetro / backdrop: `backdrop-filter: blur(8px)` su modali e toast fluttuanti.
  * Rifinitura bordo: Bordo sottile `1px solid var(--border)` con micro-accento su `:focus` o card selezionata.

---

## 3. I 6 Stati Obbligatori di Ogni Componente

Nessun componente, modale o card può essere considerato completo senza gestire esplicitamente i suoi 6 stati:

1. **Idle / Default**: Stato normale con dati caricati.
2. **Loading**:
   * Breve (<200ms): indicatore spinner inline o stato disattivato con testo "Caricamento...".
   * Lungo (>500ms): skeleton loader o progress bar chiara.
3. **Empty (Stato Vuoto)**:
   * **Vietato** limitarsi a stampare "Nessun dato".
   * Fornire: icona a tema, titolo amichevole, spiegazione di 1 riga e **un'azione primaria** (es: *"+ Collega il tuo primo Vault"*).
4. **Error (Stato Errore)**:
   * Sfondo tenue `rgba(239, 68, 68, 0.1)`, bordo rosso `rgba(239, 68, 68, 0.3)`.
   * Messaggio diagnostico chiaro con indicazione per risolvere e pulsante **"Riprova"**.
5. **Partial / Stale**:
   * Dati offline o parzialmente sincronizzati segnalati con badge giallo/ambra (`--warning`) e timestamp dell'ultimo aggiornamento.
6. **Success Feedback**:
   * Conferma dell'azione eseguita con toast fluttuante non invasivo (`showToast(...)`) della durata di 4 secondi.

---

## 4. Linee Guida per i Componenti

### Pulsanti (Buttons)
* **Primario (`.btn-primary` / `.btn-success`)**: Sfondo pieno accento (`var(--primary)` o `var(--success)`), testo a contrasto (`#020817`), font-weight 600.
* **Secondario / Ghost (`.btn-secondary` / `.btn-outline-*`)**: Sfondo trasparente o scuro con bordo discreto. Usato per annulla, chiudi o azioni secondarie.
* **Distruttivo (`.btn-danger` / `.btn-destructive`)**: Riservato a formattazione dischi, revoca chiavi, rimozione nodi.
* **Regola "No Silent Disabled Button"**: Non disabilitare mai un bottone senza spiegare all'utente perché è disabilitato. Piuttosto, lascia il bottone cliccabile e mostra un messaggio di convalida o tooltip sui campi mancanti.

### Form & Input
* Etichette (`<label>`) sempre visibili sopra l'input con dimensione `12px` e `font-weight: 600`.
* Placeholder usato come suggerimento esplicativo (es: `"es. nbp_live_xxxxxxxx"`), mai in sostituzione della label.
* Micro-istruzioni (`<small>`) posizionate immediatamente sotto il campo per indicare esattamente dove reperire il dato o le opzioni ammesse.

### Azioni Distruttive & Conferme
* Per operazioni critiche su storage, partizioni o chiavi mesh:
  * Mostrare modale di conferma dedicato con icona d'avviso rossa ⚠️.
  * Richiedere la conferma digitata (es. digitare il nome del vault o la parola "ELIMINA") per evitare click accidentali.

### Card Selezionabili (Pattern CSS `:has()`)
* Quando una card contiene una checkbox o un radio button, utilizzare il selettore CSS `:has(input:checked)` per evidenziare automaticamente il bordo con `var(--primary)` e applicare un bagliore tenue, senza richiedere logica JavaScript aggiuntiva.

---

## 5. Mobile & Responsive (< 640px)
* Modali a tutto schermo o con margini minimi laterali (12px).
* Le tabelle dense devono collassare in card list o abilitare lo scroll orizzontale esplicito con touch fluido.
* Target di tocco minimo: bottoni e controlli interattivi devono avere altezza minima di `40px` (preferibilmente `44px`) per uso agevole da smartphone.
