package org.thornex.musicparty.service;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.function.Supplier;

@Service
public class RoomStateMutationService {

    private final TransactionTemplate transactionTemplate;
    private final DatabaseWriteExecutor databaseWriteExecutor;

    public RoomStateMutationService(@Autowired(required = false) PlatformTransactionManager transactionManager) {
        this(transactionManager, null);
    }

    @Autowired
    public RoomStateMutationService(@Autowired(required = false) PlatformTransactionManager transactionManager,
                                    DatabaseWriteExecutor databaseWriteExecutor) {
        this.transactionTemplate = transactionManager == null ? null : new TransactionTemplate(transactionManager);
        this.databaseWriteExecutor = databaseWriteExecutor;
    }

    public void runInTransaction(Runnable mutation) {
        runWrite(() -> {
            if (transactionTemplate == null) {
                mutation.run();
            } else {
                transactionTemplate.executeWithoutResult(status -> mutation.run());
            }
            return null;
        });
    }

    public <T> T supplyInTransaction(Supplier<T> mutation) {
        return runWrite(() -> transactionTemplate == null ? mutation.get() : transactionTemplate.execute(status -> mutation.get()));
    }

    private <T> T runWrite(java.util.concurrent.Callable<T> operation) {
        return databaseWriteExecutor == null ? callDirectly(operation) : databaseWriteExecutor.call(operation);
    }

    private <T> T callDirectly(java.util.concurrent.Callable<T> operation) {
        try {
            return operation.call();
        } catch (Exception ex) {
            if (ex instanceof RuntimeException runtimeException) {
                throw runtimeException;
            }
            throw new IllegalStateException("Room state mutation failed", ex);
        }
    }
}
