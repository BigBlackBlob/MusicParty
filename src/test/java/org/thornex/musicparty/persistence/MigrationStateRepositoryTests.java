package org.thornex.musicparty.persistence;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class MigrationStateRepositoryTests {

    @Test
    void inMemoryMigrationStateRepositoryTracksCompletion() {
        InMemoryMigrationStateRepository repository = new InMemoryMigrationStateRepository();

        assertThat(repository.isCompleted("legacy.rooms.json")).isFalse();

        repository.markCompleted("legacy.rooms.json");

        assertThat(repository.isCompleted("legacy.rooms.json")).isTrue();
        assertThat(repository.isCompleted("legacy.queue-data.json")).isFalse();
    }
}
