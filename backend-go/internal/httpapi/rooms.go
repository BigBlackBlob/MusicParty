package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/go-chi/chi/v5"
)

type RoomAPI struct{ service *roomdomain.Service }

func NewRoomAPI(service *roomdomain.Service) *RoomAPI { return &RoomAPI{service: service} }
func (api *RoomAPI) Routes(r chi.Router) {
	r.Get("/api/rooms", Adapt(api.list))
	r.Put("/api/rooms/{roomId}", Adapt(api.update))
	r.Delete("/api/rooms/{roomId}", Adapt(api.delete))
	r.Post("/api/rooms/{roomId}/invites", Adapt(api.createInvite))
	r.Get("/api/rooms/{roomId}/invites", Adapt(api.invites))
	r.Delete("/api/rooms/{roomId}/invites/{inviteId}", Adapt(api.revokeInvite))
	r.Get("/api/rooms/{roomId}/members", Adapt(api.members))
	r.Delete("/api/rooms/{roomId}/members/{publicId}", Adapt(api.removeMember))
	r.Post("/api/rooms/{roomId}/owners/{publicId}", Adapt(api.makeOwner))
	r.Delete("/api/rooms/{roomId}/owners/{publicId}", Adapt(api.removeOwner))
}
func (api *RoomAPI) list(w http.ResponseWriter, r *http.Request) error {
	value, err := api.service.List(r.Context(), sessionToken(r))
	if err != nil {
		return err
	}
	writeJSON(w, value)
	return nil
}
func (api *RoomAPI) update(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: 400, Message: "Invalid request body"}
	}
	value, err := api.service.Update(r.Context(), chi.URLParam(r, "roomId"), sessionToken(r), request.Name)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, account.ErrUnknownSession) {
			status = http.StatusUnauthorized
		} else if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		} else if err.Error() == "No permission to update room" {
			status = http.StatusForbidden
		}
		writeErrorJSON(w, status, map[string]string{"message": err.Error()})
		return nil
	}
	writeJSON(w, value)
	return nil
}
func (api *RoomAPI) delete(w http.ResponseWriter, r *http.Request) error {
	deleted, err := api.service.Delete(r.Context(), chi.URLParam(r, "roomId"), sessionToken(r))
	if err != nil {
		if errors.Is(err, account.ErrUnknownSession) {
			return unauthorized(w)
		}
		return err
	}
	if !deleted {
		writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "No permission to delete room"})
		return nil
	}
	writeJSON(w, map[string]string{"message": "ROOM DELETED"})
	return nil
}
func (api *RoomAPI) createInvite(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Label string `json:"label"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSONBody(r, &request); err != nil {
			return &APIError{Status: 400, Message: "Invalid request body"}
		}
	}
	value, err := api.service.CreateInvite(r.Context(), chi.URLParam(r, "roomId"), sessionToken(r), request.Label)
	return roomResult(w, value, err)
}
func (api *RoomAPI) invites(w http.ResponseWriter, r *http.Request) error {
	value, err := api.service.ListInvites(r.Context(), chi.URLParam(r, "roomId"), sessionToken(r))
	return roomResult(w, value, err)
}
func (api *RoomAPI) revokeInvite(w http.ResponseWriter, r *http.Request) error {
	changed, err := api.service.RevokeInvite(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "inviteId"), sessionToken(r))
	if err != nil {
		return roomResult(w, nil, err)
	}
	if !changed {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
	return nil
}
func (api *RoomAPI) members(w http.ResponseWriter, r *http.Request) error {
	value, err := api.service.Members(r.Context(), chi.URLParam(r, "roomId"), sessionToken(r))
	return roomResult(w, value, err)
}
func (api *RoomAPI) removeMember(w http.ResponseWriter, r *http.Request) error {
	changed, err := api.service.RemoveMember(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "publicId"), sessionToken(r))
	return membershipResult(w, changed, err)
}
func (api *RoomAPI) makeOwner(w http.ResponseWriter, r *http.Request) error {
	changed, err := api.service.SetOwner(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "publicId"), sessionToken(r), true)
	return membershipResult(w, changed, err)
}
func (api *RoomAPI) removeOwner(w http.ResponseWriter, r *http.Request) error {
	changed, err := api.service.SetOwner(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "publicId"), sessionToken(r), false)
	return membershipResult(w, changed, err)
}
func roomResult(w http.ResponseWriter, value any, err error) error {
	if err == nil {
		writeJSON(w, value)
		return nil
	}
	if err.Error() == "Forbidden" {
		writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "Forbidden"})
		return nil
	}
	return err
}
func membershipResult(w http.ResponseWriter, changed bool, err error) error {
	if err != nil {
		if err.Error() == "Forbidden" {
			writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "Forbidden"})
			return nil
		}
		if err.Error() == "A room must retain an owner" {
			writeErrorJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return nil
		}
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		return err
	}
	if !changed {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
	return nil
}
