package org.thornex.musicparty.persistence;

import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryMigrationStateRepository implements MigrationStateRepository {

    private final Set<String> completed = ConcurrentHashMap.newKeySet();

    @Override
    public boolean isCompleted(String migrationKey) {
        return completed.contains(migrationKey);
    }

    @Override
    public void markCompleted(String migrationKey) {
        completed.add(migrationKey);
    }
}
