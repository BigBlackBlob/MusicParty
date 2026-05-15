package org.thornex.musicparty.persistence;

public interface MigrationStateRepository {
    boolean isCompleted(String migrationKey);
    void markCompleted(String migrationKey);
}
