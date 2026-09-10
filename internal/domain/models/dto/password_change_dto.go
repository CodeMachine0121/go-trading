package dto

// PasswordChangeDto is one attempt to replace a password: the one in force now, and
// the one meant to take its place.
//
// There is no field for who is changing it, and that absence is deliberate. Who is
// asking comes from the proof of identity on the request, never from the request's
// content — a field here would be a field somebody could fill in with a stranger's
// identifier.
type PasswordChangeDto struct {
	CurrentPassword string
	NewPassword     string
}
