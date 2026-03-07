// Package mock provides consistent, client-ready mock data for gRPC responses.
//
// Use EmptyDiff() when an RPC has no user-data updates to sync.
// Use BaselineDiff(userID) when the client should receive a full baseline state
// (e.g. after RegisterUser, Auth, or GameStart); it returns the same table set
// and JSON shapes as userdata.DefaultUserDataJSON so the client stays in sync.
//
// Constants (DefaultUserID, DefaultBackupToken, DefaultBirthYear/Month, etc.)
// keep profile and user-related response fields consistent across UserService.
package mock
