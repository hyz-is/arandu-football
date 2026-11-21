// Package football is a client for the API-Football v3 service
// (https://www.api-football.com/documentation-v3) and an in-memory stand-in for
// it.
//
// It reads leagues, teams and fixtures, and nothing else. It has no endpoint
// for odds or predictions: the package exists to follow matches and their
// scores, and the surface of the service that concerns betting is left out on
// purpose.
//
// [New] builds the client for an account's API key. [NewFake] builds a [Source]
// that holds its data in memory, so an application's tests never reach the
// network. Both satisfy [Source], which is what an application should depend on.
//
// The package imports only the standard library and registers nothing: there is
// no Module, no route and no table.
package football
