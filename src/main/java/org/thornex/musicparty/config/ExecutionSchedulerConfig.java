package org.thornex.musicparty.config;

import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.binder.jvm.ExecutorServiceMetrics;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.beans.factory.annotation.Qualifier;
import reactor.core.scheduler.Scheduler;
import reactor.core.scheduler.Schedulers;

import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

@Configuration
public class ExecutionSchedulerConfig {
    @Bean(destroyMethod = "shutdown")
    ThreadPoolExecutor dbWriteExecutor(AppProperties properties, MeterRegistry registry) {
        return executor("db-write", 1, properties.getPerformance().getDbWriteQueueCapacity(), registry);
    }

    @Bean(destroyMethod = "shutdown")
    ThreadPoolExecutor dbReadExecutor(AppProperties properties, MeterRegistry registry) {
        return executor("db-read", properties.getPerformance().getDbReadThreads(), properties.getPerformance().getDbReadQueueCapacity(), registry);
    }

    @Bean(destroyMethod = "shutdown")
    ThreadPoolExecutor fileIoExecutor(AppProperties properties, MeterRegistry registry) {
        return executor("file-io", properties.getPerformance().getFileIoThreads(), properties.getPerformance().getFileIoQueueCapacity(), registry);
    }

    @Bean(destroyMethod = "shutdown")
    ThreadPoolExecutor subprocessExecutor(AppProperties properties, MeterRegistry registry) {
        return executor("subprocess", properties.getPerformance().getSubprocessThreads(), properties.getPerformance().getSubprocessQueueCapacity(), registry);
    }

    @Bean(destroyMethod = "shutdown")
    ThreadPoolExecutor imageCpuExecutor(AppProperties properties, MeterRegistry registry) {
        return executor("image-cpu", properties.getPerformance().getImageCpuThreads(), properties.getPerformance().getImageCpuQueueCapacity(), registry);
    }

    @Bean(destroyMethod = "dispose")
    Scheduler dbReadScheduler(@Qualifier("dbReadExecutor") ThreadPoolExecutor dbReadExecutor) {
        return Schedulers.fromExecutorService(dbReadExecutor);
    }

    @Bean(destroyMethod = "dispose")
    Scheduler fileIoScheduler(@Qualifier("fileIoExecutor") ThreadPoolExecutor fileIoExecutor) {
        return Schedulers.fromExecutorService(fileIoExecutor);
    }

    @Bean(destroyMethod = "dispose")
    Scheduler subprocessScheduler(@Qualifier("subprocessExecutor") ThreadPoolExecutor subprocessExecutor) {
        return Schedulers.fromExecutorService(subprocessExecutor);
    }

    @Bean(destroyMethod = "dispose")
    Scheduler imageCpuScheduler(@Qualifier("imageCpuExecutor") ThreadPoolExecutor imageCpuExecutor) {
        return Schedulers.fromExecutorService(imageCpuExecutor);
    }

    private ThreadPoolExecutor executor(String name, int threads, int queueCapacity, MeterRegistry registry) {
        int poolSize = Math.max(1, threads);
        ThreadPoolExecutor executor = new ThreadPoolExecutor(poolSize, poolSize, 0, TimeUnit.MILLISECONDS,
                new ArrayBlockingQueue<>(Math.max(1, queueCapacity)), runnable -> {
                    Thread thread = new Thread(runnable, "mp-" + name);
                    thread.setDaemon(true);
                    return thread;
                },
                new ThreadPoolExecutor.AbortPolicy());
        ExecutorServiceMetrics.monitor(registry, executor, "musicparty.executor", java.util.List.of(io.micrometer.core.instrument.Tag.of("name", name)));
        return executor;
    }
}
