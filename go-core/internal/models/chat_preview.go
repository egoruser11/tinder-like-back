package models

// ChatPreview is a chat together with the other participant's public profile.
type ChatPreview struct {
	Chat
	PartnerUserID   int64  `json:"partner_user_id"`
	PartnerFullName string `json:"partner_full_name"`
}
