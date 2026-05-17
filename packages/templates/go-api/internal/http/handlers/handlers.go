package handlers

type Set struct {
	App    *AppHandler
	Health *HealthHandler
	Auth   *AuthHandler
	Users  *UserHandler
}
