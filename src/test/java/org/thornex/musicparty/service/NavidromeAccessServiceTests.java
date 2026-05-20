package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.persistence.InMemoryUserAccountRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;

import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;

class NavidromeAccessServiceTests {

    @Test
    void parseAllowedNamesNormalizesAndDeduplicates() {
        Set<String> names = NavidromeAccessService.parseAllowedNames(" Nirotiy,alice, NIROTIY ,, Alice ");

        assertThat(names).containsExactlyInAnyOrder("nirotiy", "alice");
    }

    @Test
    void parseAllowedNamesReturnsEmptySetForBlankInput() {
        assertThat(NavidromeAccessService.parseAllowedNames("  , , ")).isEmpty();
        assertThat(NavidromeAccessService.parseAllowedNames(null)).isEmpty();
    }

    @Test
    void parseAllowedNamesKeepsWildcardForLocalDevelopment() {
        Set<String> names = NavidromeAccessService.parseAllowedNames("*, Alice");

        assertThat(names).containsExactlyInAnyOrder("*", "alice");
    }

    @Test
    void allowsAllNamedUsersWhenWildcardIsConfigured() {
        AppProperties properties = new AppProperties();
        properties.getNavidrome().setAllowedUsers("*");
        NavidromeAccessService service = new NavidromeAccessService(properties, null,
                new AccountService(new InMemoryUserAccountRepository(), new InMemoryUserProfileRepository()));

        assertThat(service.allowsAllNamedUsers()).isTrue();
    }
}
