package main

import (
	"strings"
	"testing"
)

// The Administrators group is the one group whose membership is itself an
// authority, so its page states what that authority is and what it is not.
// Every claim below is the web statement of a rule owned by
// docs/01-identity-and-roles.md; the page is not allowed to invent one, and no
// other group page makes any such claim.
func TestTheAdministratorsGroupStatesItsAuthorityAndNoOtherGroupDoes(t *testing.T) {
	m := meaningFixture(t)
	// An ordinary group, so "only this group says it" can fail. One group in
	// the fixture would make the second half of this check unfalsifiable.
	if err := m.bus.SetGroup("admin@h", "@ops", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}

	protected := m.get("/group?name=%40administrators")
	ordinary := m.get("/group?name=%40ops")
	if !strings.Contains(ordinary, "<code>@ops</code>") {
		t.Fatalf("the ordinary group page did not render, so nothing below is proved: %s", ordinary)
	}

	// docs/01-identity-and-roles.md#daemon-administrators and #users-and-profiles:
	// the directory, registration, and editing below one's own level.
	// #groups: ordinary group membership, including Maintainer groups.
	granted := []string{
		"See the whole user directory, register new users, and edit, deactivate or reactivate users below your own level.",
		"Change the membership of any ordinary group, including one assigned as a resource&rsquo;s Maintainer",
		"you may add yourself, or a user you created, without asking that resource&rsquo;s owner.",
		"Read the full membership of every group.",
	}
	// The three limits, each owned by a different part of the same document.
	withheld := []string{
		"An Administrator cannot edit the daemon Owner or another Administrator, and cannot grant either position.",
		"Administering the node is not managing its resources.",
		"Only the daemon Owner changes who is in it.",
	}
	for _, claim := range append(append([]string{}, granted...), withheld...) {
		if !strings.Contains(protected, claim) {
			t.Errorf("the Administrators page does not state %q", claim)
		}
		if strings.Contains(ordinary, claim) {
			t.Errorf("an ordinary group page claims an authority it does not confer: %q", claim)
		}
	}

	// Both halves are on the page itself, not folded into the help popover:
	// a right nobody opens is a right nobody reads.
	rights := section(t, protected, `<section class="dashboard-section admin-rights">`, "</section>")
	for _, heading := range []string{"<h2>What membership grants</h2>", "<h2>What it does not grant</h2>"} {
		if !strings.Contains(rights, heading) {
			t.Errorf("the rights section is missing %s: %s", heading, rights)
		}
	}
	if strings.Contains(rights, "popover") {
		t.Errorf("the rights are inside a popover rather than on the page: %s", rights)
	}
	if strings.Contains(ordinary, "admin-rights") {
		t.Errorf("an ordinary group page carries the rights section: %s", ordinary)
	}
}
