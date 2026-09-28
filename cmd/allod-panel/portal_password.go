package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/asfaltobollente/allod/internal/helper"
	"github.com/asfaltobollente/allod/internal/panel"
	"github.com/asfaltobollente/allod/internal/state"
)

// HandlePortalSetPassword handles setting or resetting a family member's password.
// For security (V1), it strictly requires a valid, non-expired invitation token.
// Direct username-only requests without a token are rejected.
func HandlePortalSetPassword(dbPath string, ensureSysUser func(client *helper.Client, username string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Token       string `json:"token"`
			Username    string `json:"username"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		token := strings.TrimSpace(req.Token)
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Token di invito non specificato o non valido"})
			return
		}

		if len(req.NewPassword) < 8 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La nuova password deve contenere almeno 8 caratteri"})
			return
		}

		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore database: " + err.Error()})
			return
		}
		defer st.Close()

		member, errVal := st.ValidateResetToken(token)
		if errVal != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errVal.Error()})
			return
		}
		targetUser := member.Username

		// Consume token before creating session / setting password to prevent replay attacks
		if errConsume := st.ConsumeResetToken(token); errConsume != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore utilizzo token: " + errConsume.Error()})
			return
		}

		// Set Samba password via root helper
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if ensureSysUser != nil {
			if errEnsure := ensureSysUser(&client, targetUser); errEnsure != nil {
				log.Printf("[portal] Avviso ensureSystemUser: %v", errEnsure)
			}
		}

		errSmb := client.SetSambaPassword(targetUser, req.NewPassword)
		if errSmb != nil && ensureSysUser != nil {
			// Retry once after ensuring user
			_ = ensureSysUser(&client, targetUser)
			errSmb = client.SetSambaPassword(targetUser, req.NewPassword)
		}

		if errSmb != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore aggiornamento password Samba: %v", errSmb),
			})
			return
		}

		// Save password hash into family member record in state.db
		if salt, errSalt := panel.GenerateSalt(); errSalt == nil {
			hash := panel.HashPassword(req.NewPassword, salt)
			_ = st.SetFamilyMemberPassword(targetUser, hash, hex.EncodeToString(salt))
		}

		// Create session cookie so the user is immediately authenticated in the portal
		sessToken, errSess := st.CreateSession("family", targetUser, panel.SessionDuration)
		if errSess == nil {
			panel.SetSessionCookie(w, panel.FamilyCookieName, sessToken, int(panel.SessionDuration.Seconds()))
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Password personale per '%s' impostata con successo!", targetUser),
			Data: map[string]interface{}{
				"username": targetUser,
			},
		})
	}
}
