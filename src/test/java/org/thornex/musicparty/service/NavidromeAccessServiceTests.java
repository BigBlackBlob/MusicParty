package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;

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
}
