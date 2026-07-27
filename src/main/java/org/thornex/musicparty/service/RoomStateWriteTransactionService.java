package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.persistence.PersistedPlaybackState;
import org.thornex.musicparty.persistence.PlaybackStateRepository;
import org.thornex.musicparty.persistence.QueueRepository;

import java.util.List;

/**
 * Starts the transaction only after DatabaseWriteExecutor has admitted the
 * work onto its single SQLite writer thread.  Starting it on a caller thread
 * would reserve a pooled connection while that thread waits for the writer.
 */
@Service
public class RoomStateWriteTransactionService {
    private final QueueRepository queueRepository;
    private final PlaybackStateRepository playbackStateRepository;

    public RoomStateWriteTransactionService(QueueRepository queueRepository,
                                            PlaybackStateRepository playbackStateRepository) {
        this.queueRepository = queueRepository;
        this.playbackStateRepository = playbackStateRepository;
    }

    @Transactional
    public void flush(String roomId,
                      List<MusicQueueItem> queueItems,
                      List<Music> historyItems,
                      PersistedPlaybackState playbackState) {
        queueRepository.synchronizeQueue(roomId, queueItems);
        queueRepository.replaceHistory(roomId, historyItems);
        playbackStateRepository.upsert(playbackState);
    }
}
