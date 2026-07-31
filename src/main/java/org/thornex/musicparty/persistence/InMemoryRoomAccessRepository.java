package org.thornex.musicparty.persistence;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryRoomAccessRepository implements RoomAccessRepository {
    private final Map<String, PersistedRoomMembership> memberships = new ConcurrentHashMap<>();
    private final Map<String, PersistedRoomInvite> invites = new ConcurrentHashMap<>();
    private String key(String roomId, String publicId) { return roomId + "\u0000" + publicId; }
    public void upsertMembership(PersistedRoomMembership m) { memberships.put(key(m.roomId(),m.publicId()),m); }
    public Optional<PersistedRoomMembership> findMembership(String r,String p){return Optional.ofNullable(memberships.get(key(r,p)));}
    public List<PersistedRoomMembership> listMemberships(String r){return memberships.values().stream().filter(m->m.roomId().equals(r)).toList();}
    public int countOwners(String r){return (int)listMemberships(r).stream().filter(PersistedRoomMembership::owner).count();}
    public void deleteMembership(String r,String p){memberships.remove(key(r,p));}
    public void createInvite(PersistedRoomInvite i){invites.put(i.id(),i);}
    public Optional<PersistedRoomInvite> findInviteById(String id){return Optional.ofNullable(invites.get(id));}
    public Optional<PersistedRoomInvite> findInviteBySecretHash(String h){return invites.values().stream().filter(i->i.secretHash().equals(h)).findFirst();}
    public List<PersistedRoomInvite> listInvites(String r){return invites.values().stream().filter(i->i.roomId().equals(r)).toList();}
    public synchronized boolean consumeInvite(String id,String p,long at){var i=invites.get(id);if(i==null||!i.active(at))return false;invites.put(id,new PersistedRoomInvite(i.id(),i.roomId(),i.createdByPublicId(),i.secretHash(),i.label(),i.expiresAt(),i.maxUses(),at,p,i.revokedAt(),i.createdAt()));return true;}
    public synchronized boolean revokeInvite(String id,long at){var i=invites.get(id);if(i==null||i.revokedAt()!=null)return false;invites.put(id,new PersistedRoomInvite(i.id(),i.roomId(),i.createdByPublicId(),i.secretHash(),i.label(),i.expiresAt(),i.maxUses(),i.usedAt(),i.usedByPublicId(),at,i.createdAt()));return true;}
    public synchronized boolean revokeInvite(String roomId,String id,long at){var i=invites.get(id);if(i==null||!i.roomId().equals(roomId)||i.revokedAt()!=null)return false;invites.put(id,new PersistedRoomInvite(i.id(),i.roomId(),i.createdByPublicId(),i.secretHash(),i.label(),i.expiresAt(),i.maxUses(),i.usedAt(),i.usedByPublicId(),at,i.createdAt()));return true;}
}
