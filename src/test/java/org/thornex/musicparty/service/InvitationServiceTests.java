package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.persistence.InMemoryRoomAccessRepository;
import org.thornex.musicparty.persistence.InMemoryUserAccountRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class InvitationServiceTests {

    @Test
    void inviteCanBeRedeemedExactlyOnceAndCreatesMembership() {
        InMemoryRoomAccessRepository access = new InMemoryRoomAccessRepository();
        InvitationService service = new InvitationService(
                access,
                new InMemoryUserProfileRepository(),
                new InMemoryUserAccountRepository(),
                new RoomStateMutationService(null)
        );

        InvitationService.CreatedInvite invite = service.create("room-1", "u_owner", " Friends ");
        AccountSession session = service.redeem(invite.secret(), " Alice ");

        assertThat(invite.label()).isEqualTo("Friends");
        assertThat(session.displayName()).isEqualTo("Alice");
        assertThat(session.role()).isEqualTo("MEMBER");
        assertThat(access.findMembership("room-1", session.publicId())).isPresent();
        assertThatThrownBy(() -> service.redeem(invite.secret(), "Mallory"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessage("Invitation is invalid or expired");
    }

    @Test
    void revokeIsScopedToTheOwningRoom() {
        InMemoryRoomAccessRepository access = new InMemoryRoomAccessRepository();
        InvitationService service = new InvitationService(
                access,
                new InMemoryUserProfileRepository(),
                new InMemoryUserAccountRepository(),
                new RoomStateMutationService(null)
        );
        InvitationService.CreatedInvite invite = service.create("room-1", "u_owner", null);

        assertThat(service.revoke("room-2", invite.id())).isFalse();
        assertThat(service.metadata(invite.secret()).valid()).isTrue();
        assertThat(service.revoke("room-1", invite.id())).isTrue();
        assertThat(service.metadata(invite.secret()).valid()).isFalse();
    }
}
