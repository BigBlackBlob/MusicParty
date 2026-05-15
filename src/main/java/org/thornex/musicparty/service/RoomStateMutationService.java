package org.thornex.musicparty.service;

import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.function.Supplier;

@Service
public class RoomStateMutationService {

    private final TransactionTemplate transactionTemplate;

    @Autowired
    public RoomStateMutationService(@Autowired(required = false) PlatformTransactionManager transactionManager) {
        this.transactionTemplate = transactionManager == null ? null : new TransactionTemplate(transactionManager);
    }

    public void runInTransaction(Runnable mutation) {
        if (transactionTemplate == null) {
            mutation.run();
            return;
        }
        transactionTemplate.executeWithoutResult(status -> mutation.run());
    }

    public <T> T supplyInTransaction(Supplier<T> mutation) {
        if (transactionTemplate == null) {
            return mutation.get();
        }
        return transactionTemplate.execute(status -> mutation.get());
    }
}
