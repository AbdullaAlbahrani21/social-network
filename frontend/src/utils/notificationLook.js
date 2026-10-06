const LOOKS = {
  message_received: { icon: 'messages', tone: 'purple' },

  follow_request: { icon: 'profile', tone: 'pink' },
  follow_accepted: { icon: 'profile', tone: 'pink' },

  post_liked: { icon: 'heart', tone: 'muted' },
  comment_created: { icon: 'text', tone: 'muted' },

  group_invite: { icon: 'groups', tone: 'muted' },
  group_invite_declined: { icon: 'groups', tone: 'muted' },
  group_join_request: { icon: 'groups', tone: 'muted' },
  group_event_created: { icon: 'calendar', tone: 'muted' },
}

const FALLBACK = { icon: 'notifications', tone: 'muted' }

export function notificationLook(type) {
  return LOOKS[type] ?? FALLBACK
}
