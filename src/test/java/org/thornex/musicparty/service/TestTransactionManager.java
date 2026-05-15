package org.thornex.musicparty.service;

import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.AbstractPlatformTransactionManager;
import org.springframework.transaction.support.DefaultTransactionStatus;

class TestTransactionManager extends AbstractPlatformTransactionManager {

    @Override
    protected Object doGetTransaction() {
        return new Object();
    }

    @Override
    protected void doBegin(Object transaction, TransactionDefinition definition) {
        // no-op
    }

    @Override
    protected void doCommit(DefaultTransactionStatus status) {
        // no-op
    }

    @Override
    protected void doRollback(DefaultTransactionStatus status) {
        // no-op
    }
}
