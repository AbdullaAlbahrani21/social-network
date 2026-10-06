<script setup>
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute } from "vue-router";
import { api } from "../api";
import NavIcon from "../components/NavIcon.vue";
import SegmentedToggle from "../components/SegmentedToggle.vue";
import { useAuthStore } from "../stores/auth";
import { formatTimestamp } from "../utils/display";

const route = useRoute();
const auth = useAuthStore();

const profile = ref(null);
const loading = ref(false);
const error = ref("");
const updatingVisibility = ref(false);
const followActionLoading = ref(false);
const actionError = ref("");
const followRequests = ref([]);

// ?? not ||, as in api.js's BASE_URL: an empty value is the deliberate "same origin" setting.
const backendURL =
    import.meta.env.VITE_API_BASE_URL ??
    "http://localhost:8080";

const user = computed(() => profile.value?.user ?? null);

const PRIVACY_LABELS = {
    public: "Public",
    almost_private: "Followers",
    private: "Selected followers",
};

function assetURL(path) {
    if (!path) {
        return "";
    }

    if (path.startsWith("http://") || path.startsWith("https://")) {
        return path;
    }

    return `${backendURL}${path}`;
}

function displayName(person) {
    if (person.nickname) {
        return person.nickname;
    }

    return `${person.firstName} ${person.lastName}`;
}

function plural(count, one, many) {
    return count === 1 ? one : many;
}

// date_of_birth arrives as midnight UTC; reading it in UTC keeps it from shifting a day for anyone west of Greenwich.
function formatBirthday(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return value;
    }

    return date.toLocaleDateString(undefined, {
        dateStyle: "long",
        timeZone: "UTC",
    });
}

async function loadProfile(keepCurrent = false) {
    const refresh = keepCurrent === true;
    loading.value = !refresh;
    error.value = "";
    if (!refresh) {
        profile.value = null;
    }

    try {
        profile.value = await api.get(
            `/api/profile/${route.params.id}`,
        );

        if (profile.value.isOwn) {
            followRequests.value = await api.get("/api/follow-requests");
        } else {
            followRequests.value = [];
        }
    } catch (err) {
        error.value = err.message;
    } finally {
        loading.value = false;
    }
}

async function toggleVisibility() {
    if (!profile.value?.isOwn || updatingVisibility.value) {
        return;
    }

    updatingVisibility.value = true;
    error.value = "";

    const nextValue = !profile.value.user.isPublic;

    try {
        const result = await api.put(
            "/api/profile/visibility",
            { isPublic: nextValue },
        );

        profile.value.user.isPublic = result.isPublic;

        if (auth.user?.id === profile.value.user.id) {
            auth.user.isPublic = result.isPublic;
        }
    } catch (err) {
        error.value = err.message;
    } finally {
        updatingVisibility.value = false;
    }
}

const confirmingVisibility = ref(false);
const visibilityButton = ref(null);
const confirmVisibilityButton = ref(null);

async function askVisibilityChange() {
    confirmingVisibility.value = true;
    await nextTick();
    confirmVisibilityButton.value?.focus();
}

async function cancelVisibilityChange() {
    confirmingVisibility.value = false;
    await nextTick();
    visibilityButton.value?.focus();
}

async function confirmVisibilityChange() {
    confirmingVisibility.value = false;
    await toggleVisibility();
    await nextTick();
    visibilityButton.value?.focus();
}

async function followUser() {
    if (!profile.value || profile.value.isOwn || followActionLoading.value) {
        return;
    }

    followActionLoading.value = true;
    actionError.value = "";

    try {
        await api.post(`/api/users/${profile.value.user.id}/follow`, {});
        await loadProfile(true);
    } catch (err) {
        actionError.value = err.message;
    } finally {
        followActionLoading.value = false;
    }
}

async function unfollowUser() {
    if (!profile.value || profile.value.isOwn || followActionLoading.value) {
        return;
    }

    followActionLoading.value = true;
    actionError.value = "";

    try {
        await api.del(`/api/users/${profile.value.user.id}/follow`);
        await loadProfile(true);
    } catch (err) {
        actionError.value = err.message;
    } finally {
        followActionLoading.value = false;
    }
}

async function respondToFollowRequest(requestId, action) {
    if (followActionLoading.value) {
        return;
    }

    followActionLoading.value = true;
    actionError.value = "";

    try {
        await api.post(`/api/follow-requests/${requestId}/${action}`, {});
        await loadProfile(true);
    } catch (err) {
        actionError.value = err.message;
    } finally {
        followActionLoading.value = false;
    }
}

const peopleDialog = ref(null);
const listKind = ref(null);

const listPeople = computed(() =>
    listKind.value && profile.value ? profile.value[listKind.value] : [],
);

function openList(kind) {
    listKind.value = kind;
    peopleDialog.value?.showModal();
}

function closeList() {
    peopleDialog.value?.close();
}

// A click that lands on the <dialog> itself, not the panel inside it, was on the backdrop.
function onDialogClick(event) {
    if (event.target === peopleDialog.value) {
        closeList();
    }
}

const failedAvatars = ref(new Set());

function showAvatar(person) {
    return person.avatarPath && !failedAvatars.value.has(person.id);
}

function dropAvatar(person) {
    failedAvatars.value = new Set(failedAvatars.value).add(person.id);
}

onMounted(loadProfile);
watch(() => route.params.id, loadProfile);
watch(() => route.params.id, () => {
    closeList();
    confirmingVisibility.value = false;
});
</script>

<template>
    <main class="profile-page">
        <p v-if="loading" class="status loading-status">Loading profile...</p>

        <section
            v-else-if="error"
            class="profile-unavailable"
        >
            <h1>Profile unavailable</h1>
            <p class="error">{{ error }}</p>
        </section>

        <template v-else-if="profile && user">
            <header class="profile-header">
                <span class="avatar-ring">
                    <img
                        v-if="showAvatar(user)"
                        class="avatar avatar-large"
                        :src="assetURL(user.avatarPath)"
                        alt=""
                        @error="dropAvatar(user)"
                    />
                    <span
                        v-else
                        class="avatar avatar-large avatar-initial"
                        aria-hidden="true"
                    >
                        {{ user.firstName.charAt(0) }}
                    </span>
                    <span
                        v-if="!profile.isOwn && !user.isPublic"
                        class="avatar-lock"
                        aria-hidden="true"
                    >
                        <NavIcon name="lock" />
                    </span>
                </span>

                <div class="identity">
                    <h1 class="nickname">{{ displayName(user) }}</h1>
                    <p class="full-name">{{ user.firstName }} {{ user.lastName }}</p>
                    <p v-if="!profile.isOwn" class="account-kind">
                        <NavIcon :name="user.isPublic ? 'globe' : 'lock'" />
                        {{ user.isPublic ? "Public profile" : "Private account" }}
                    </p>
                </div>

                <div
                    class="profile-action"
                    :class="{ 'is-confirming': confirmingVisibility }"
                >
                    <template v-if="profile.isOwn">
                        <!-- Simultaneous, not out-in: ask and cancel move focus into the incoming state on the next tick, so it must be in the DOM at once. -->
                        <Transition name="swap">
                            <div
                                v-if="confirmingVisibility"
                                class="visibility-confirm"
                                role="group"
                                aria-labelledby="visibility-question"
                                @keydown.esc="cancelVisibilityChange"
                            >
                                <p id="visibility-question" class="confirm-question">
                                    <NavIcon :name="user.isPublic ? 'lock' : 'globe'" />
                                    {{ user.isPublic ? "Make your profile private?" : "Make your profile public?" }}
                                </p>
                                <p class="confirm-detail">
                                    {{
                                        user.isPublic
                                            ? "Only people who follow you will see your posts and connections."
                                            : "Anyone will be able to see your profile, and new follows won't need your approval."
                                    }}
                                </p>
                                <div class="confirm-buttons">
                                    <button
                                        type="button"
                                        class="pill-button pill-ghost"
                                        @click="cancelVisibilityChange"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        ref="confirmVisibilityButton"
                                        type="button"
                                        class="pill-button pill-solid"
                                        @click="confirmVisibilityChange"
                                    >
                                        Confirm
                                    </button>
                                </div>
                            </div>

                            <div v-else class="visibility-control">
                                <span class="visibility-change" aria-hidden="true">
                                    {{ updatingVisibility ? "Updating..." : "Change" }}
                                </span>
                                <SegmentedToggle
                                    ref="visibilityButton"
                                    label="Profile visibility"
                                    :options="[
                                        { value: true, label: 'Public' },
                                        { value: false, label: 'Private', icon: 'lock' },
                                    ]"
                                    :model-value="user.isPublic"
                                    :disabled="updatingVisibility"
                                    :aria-busy="updatingVisibility ? 'true' : undefined"
                                    @select="(value) => value !== user.isPublic && askVisibilityChange()"
                                />
                            </div>
                        </Transition>
                    </template>

                    <button
                        v-else-if="['none', 'pending', 'accepted'].includes(profile.followStatus)"
                        type="button"
                        class="follow-button"
                        :class="[
                            `is-${profile.followStatus}`,
                            { 'is-busy': followActionLoading },
                        ]"
                        :disabled="followActionLoading"
                        :aria-busy="followActionLoading ? 'true' : undefined"
                        @click="profile.followStatus === 'none' ? followUser() : unfollowUser()"
                    >
                        <Transition name="label" mode="out-in">
                            <span v-if="followActionLoading" key="busy" class="follow-labels">
                                <span class="follow-rest">Working...</span>
                            </span>
                            <span v-else-if="profile.followStatus === 'none'" key="none" class="follow-labels">
                                <span class="follow-rest">Follow</span>
                            </span>
                            <span v-else-if="profile.followStatus === 'pending'" key="pending" class="follow-labels">
                                <span class="follow-rest">Requested</span>
                                <span class="follow-intent" aria-hidden="true">Cancel request</span>
                                <span class="visually-hidden">: cancel request</span>
                            </span>
                            <span v-else key="accepted" class="follow-labels">
                                <span class="follow-rest">Following</span>
                                <span class="follow-intent" aria-hidden="true">Unfollow</span>
                                <span class="visually-hidden">: unfollow</span>
                            </span>
                        </Transition>
                    </button>
                </div>
            </header>

            <Transition name="fade">
                <p v-if="actionError" class="error action-error" role="alert">{{ actionError }}</p>
            </Transition>

            <ul class="stats" aria-label="Profile counts">
                <li>
                    <span class="stat">
                        <span class="stat-value">
                            <Transition name="count">
                                <span :key="profile.postCount">{{ profile.postCount }}</span>
                            </Transition>
                        </span>
                        <span class="stat-label">{{ plural(profile.postCount, "post", "posts") }}</span>
                    </span>
                </li>
                <li>
                    <span v-if="profile.isRestricted" class="stat">
                        <span class="stat-value">
                            <Transition name="count">
                                <span :key="profile.followerCount">{{ profile.followerCount }}</span>
                            </Transition>
                        </span>
                        <span class="stat-label">{{ plural(profile.followerCount, "follower", "followers") }}</span>
                    </span>
                    <button
                        v-else
                        type="button"
                        class="stat stat-button"
                        aria-haspopup="dialog"
                        @click="openList('followers')"
                    >
                        <span class="stat-value">
                            <Transition name="count">
                                <span :key="profile.followerCount">{{ profile.followerCount }}</span>
                            </Transition>
                        </span>
                        <span class="stat-label">{{ plural(profile.followerCount, "follower", "followers") }}</span>
                    </button>
                </li>
                <li>
                    <span v-if="profile.isRestricted" class="stat">
                        <span class="stat-value">
                            <Transition name="count">
                                <span :key="profile.followingCount">{{ profile.followingCount }}</span>
                            </Transition>
                        </span>
                        <span class="stat-label">following</span>
                    </span>
                    <button
                        v-else
                        type="button"
                        class="stat stat-button"
                        aria-haspopup="dialog"
                        @click="openList('following')"
                    >
                        <span class="stat-value">
                            <Transition name="count">
                                <span :key="profile.followingCount">{{ profile.followingCount }}</span>
                            </Transition>
                        </span>
                        <span class="stat-label">following</span>
                    </button>
                </li>
            </ul>

            <section v-if="!profile.isRestricted" class="about" aria-label="About">
                <p v-if="user.aboutMe" class="about-me">{{ user.aboutMe }}</p>
                <dl class="details">
                    <div>
                        <dt><NavIcon name="mail" />Email</dt>
                        <dd>{{ user.email }}</dd>
                    </div>
                    <div v-if="user.dateOfBirth">
                        <dt><NavIcon name="calendar" />Birthday</dt>
                        <dd>{{ formatBirthday(user.dateOfBirth) }}</dd>
                    </div>
                </dl>
            </section>

            <Transition name="collapse">
                <section
                    v-if="profile.isOwn && followRequests.length > 0"
                    class="card requests-card"
                    aria-labelledby="requests-title"
                >
                    <h2 id="requests-title" class="card-title">
                        Follow requests
                        <span class="section-count">{{ followRequests.length }}</span>
                    </h2>

                    <TransitionGroup
                        tag="ul"
                        name="request-list"
                        class="requests"
                        @before-leave="(el) => (el.style.top = `${el.offsetTop}px`)"
                    >
                        <li
                            v-for="request in followRequests"
                            :key="request.id"
                            class="request"
                        >
                            <RouterLink :to="`/profile/${request.follower.id}`" class="person">
                                <img
                                    v-if="showAvatar(request.follower)"
                                    class="avatar"
                                    :src="assetURL(request.follower.avatarPath)"
                                    alt=""
                                    @error="dropAvatar(request.follower)"
                                />
                                <span v-else class="avatar avatar-initial" aria-hidden="true">
                                    {{ request.follower.firstName.charAt(0) }}
                                </span>
                                <span class="person-text">
                                    <span class="person-name">{{ displayName(request.follower) }}</span>
                                    <span class="person-detail">wants to follow you</span>
                                </span>
                            </RouterLink>

                            <div class="request-actions">
                                <button
                                    type="button"
                                    class="pill-button pill-ghost"
                                    :disabled="followActionLoading"
                                    @click="respondToFollowRequest(request.id, 'decline')"
                                >
                                    Decline
                                </button>
                                <button
                                    type="button"
                                    class="pill-button pill-solid"
                                    :disabled="followActionLoading"
                                    @click="respondToFollowRequest(request.id, 'accept')"
                                >
                                    Accept
                                </button>
                            </div>
                        </li>
                    </TransitionGroup>
                </section>
            </Transition>

            <section
                v-if="profile.isRestricted"
                class="locked"
                aria-labelledby="locked-title"
            >
                <span class="locked-icon"><NavIcon name="lock" /></span>
                <h2 id="locked-title" class="locked-title">This account is private</h2>
                <p class="locked-text">
                    {{
                        profile.followStatus === "pending"
                            ? `Your follow request is waiting for ${displayName(user)} to approve it.`
                            : `Follow ${displayName(user)} to see their posts, followers and who they follow.`
                    }}
                </p>
            </section>

            <section v-else class="section" aria-labelledby="posts-title">
                <h2 id="posts-title" class="section-title">Posts</h2>

                <div v-if="profile.posts.length === 0" class="empty-state">
                    <span class="empty-icon"><NavIcon name="text" /></span>
                    <p class="empty-text">
                        {{ profile.isOwn ? "You haven't posted yet." : "No visible posts yet." }}
                    </p>
                </div>

                <ul v-else class="posts">
                    <li v-for="post in profile.posts" :key="post.id">
                        <article class="post">
                            <p class="post-meta">
                                <RouterLink :to="`/posts/${post.id}`" class="post-time">
                                    <time :datetime="post.createdAt">{{ formatTimestamp(post.createdAt) }}</time>
                                </RouterLink>
                                <span aria-hidden="true">·</span>
                                <span>{{ PRIVACY_LABELS[post.privacy] || post.privacy }}</span>
                            </p>

                            <p class="post-content">{{ post.content }}</p>

                            <img
                                v-if="post.imagePath"
                                class="post-image"
                                :src="assetURL(post.imagePath)"
                                :alt="`Image posted by ${displayName(user)}`"
                                loading="lazy"
                            />
                        </article>
                    </li>
                </ul>
            </section>
        </template>

        <dialog
            ref="peopleDialog"
            class="people-dialog"
            aria-labelledby="people-dialog-title"
            @close="listKind = null"
            @click="onDialogClick"
        >
            <div v-if="listKind" class="dialog-panel">
                <header class="dialog-head">
                    <h2 id="people-dialog-title" class="visually-hidden">
                        {{ listKind === "followers" ? "Followers" : "Following" }}
                    </h2>
                    <SegmentedToggle
                        class="dialog-tabs"
                        label="Show"
                        :options="[
                            { value: 'followers', label: 'Followers' },
                            { value: 'following', label: 'Following' },
                        ]"
                        :model-value="listKind"
                        @select="(kind) => (listKind = kind)"
                    />
                    <button
                        type="button"
                        class="icon-button"
                        aria-label="Close"
                        @click="closeList"
                    >
                        <NavIcon name="close" />
                    </button>
                </header>

                <Transition name="fade" mode="out-in">
                    <p v-if="listPeople.length === 0" :key="`${listKind}-empty`" class="status dialog-empty">
                        {{
                            listKind === "followers"
                                ? "No followers yet."
                                : "Not following anyone yet."
                        }}
                    </p>

                    <ul v-else :key="listKind" class="people">
                        <li v-for="person in listPeople" :key="person.id">
                            <RouterLink
                                :to="`/profile/${person.id}`"
                                class="person person-row"
                                @click="closeList"
                            >
                                <img
                                    v-if="showAvatar(person)"
                                    class="avatar"
                                    :src="assetURL(person.avatarPath)"
                                    alt=""
                                    @error="dropAvatar(person)"
                                />
                                <span v-else class="avatar avatar-initial" aria-hidden="true">
                                    {{ person.firstName.charAt(0) }}
                                </span>
                                <span class="person-text">
                                    <span class="person-name">{{ displayName(person) }}</span>
                                    <span v-if="person.nickname" class="person-detail">
                                        {{ person.firstName }} {{ person.lastName }}
                                    </span>
                                </span>
                                <span v-if="person.id === auth.user?.id" class="person-tag">You</span>
                                <NavIcon v-else name="back" class="person-chevron" />
                            </RouterLink>
                        </li>
                    </ul>
                </Transition>
            </div>
        </dialog>
    </main>
</template>

<style scoped src="../styles/controls.css"></style>
<style scoped>
.profile-page {
    max-width: 48rem;
    margin: 0 auto;
    padding-block: 40px var(--space-7);
}

.loading-status {
    animation: appear-late 1ms 300ms both;
}

@keyframes appear-late {
    from { opacity: 0; }
    to { opacity: 1; }
}

.profile-unavailable h1 {
    margin: 0 0 var(--space-3);
}

.profile-header {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-4) 22px;
}

.avatar {
    display: block;
    flex: none;
    width: 2.5rem;
    height: 2.5rem;
    border-radius: 50%;
    object-fit: cover;
}

.avatar-initial {
    display: grid;
    place-items: center;
    background-color: var(--badge-purple-bg);
    color: var(--badge-purple-fg);
    font-size: var(--text-label);
    font-weight: 700;
    text-transform: uppercase;
}

.avatar-ring {
    position: relative;
    flex: none;
    padding: 3px;
    background: var(--panel-gradient);
    border-radius: var(--radius-pill);
}

.avatar-large {
    width: 5.25rem;
    height: 5.25rem;
    border: 3px solid var(--surface);
}

.avatar-large.avatar-initial {
    font-family: var(--font-display);
    font-size: 1.75rem;
    font-weight: 700;
}

.avatar-lock {
    position: absolute;
    right: -2px;
    bottom: 2px;
    display: grid;
    place-items: center;
    width: 1.75rem;
    height: 1.75rem;
    background-color: var(--surface);
    border-radius: 50%;
    box-shadow: 0 4px 10px -4px color-mix(in srgb, var(--primary) 45%, transparent);
    color: var(--primary);
}

.avatar-lock .nav-icon {
    width: 0.85rem;
    height: 0.85rem;
}

.identity {
    flex: 1 1 10rem;
    min-width: 0;
}

.nickname {
    margin: 0;
    font-size: 1.75rem;
    font-weight: 800;
    line-height: 1.2;
    overflow-wrap: anywhere;
}

.full-name {
    margin: 2px 0 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
    font-weight: 500;
}

.account-kind {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    margin: 6px 0 0;
    color: var(--text-muted);
    font-size: var(--text-meta);
    font-weight: 600;
}

.account-kind .nav-icon {
    width: 0.8rem;
    height: 0.8rem;
}

.profile-action {
    position: relative;
    flex: none;
    align-self: flex-start;
    margin-top: 6px;
}

.profile-action.is-confirming {
    display: flex;
    flex-basis: 100%;
    justify-content: flex-end;
    margin-top: 0;
}

.visibility-control {
    display: flex;
    align-items: center;
    gap: 12px;
}

.visibility-change {
    color: var(--text-placeholder);
    font-size: var(--text-meta);
    font-weight: 700;
}

/* A fixed width, not a percentage: the action slot sizes to its content, so a percentage would resolve against the text and wrap under the name. */
.visibility-confirm {
    width: 21rem;
    max-width: 100%;
    padding: 16px 18px;
    background-color: var(--surface);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-card);
}

.confirm-question {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-weight: 700;
}

.confirm-question .nav-icon {
    width: 1rem;
    height: 1rem;
    color: var(--primary);
}

.confirm-detail {
    margin: 6px 0 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
    line-height: 1.5;
}

.confirm-buttons {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: 14px;
}

.swap-enter-active,
.swap-leave-active {
    transform-origin: right top;
    transition:
        opacity var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.swap-leave-active {
    position: absolute;
    top: 0;
    right: 0;
}

.swap-enter-from,
.swap-leave-to {
    opacity: 0;
    transform: scale(0.94);
}

.pill-button {
    min-height: 2.25rem;
    padding: 0 18px;
    border: 0;
    border-radius: var(--radius-pill);
    font-size: var(--text-label);
    font-weight: 700;
    cursor: pointer;
    transition:
        background-color var(--duration-fast) var(--ease-out),
        color var(--duration-fast) var(--ease-out),
        box-shadow var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.pill-button:disabled {
    cursor: progress;
    opacity: 0.6;
}

.pill-solid {
    background-color: var(--primary);
    box-shadow: var(--shadow-primary);
    color: var(--surface);
}

.pill-solid:hover:not(:disabled) {
    background-color: var(--primary-hover);
    box-shadow: var(--shadow-primary-lift);
    transform: translateY(-1px);
}

.pill-ghost {
    background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
    color: var(--text-muted);
}

.pill-ghost:hover:not(:disabled) {
    background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
    color: var(--text);
}

.pill-button:focus-visible {
    outline: 2px solid var(--primary);
    outline-offset: 2px;
}

.follow-button {
    position: relative;
    display: inline-grid;
    place-items: center;
    min-width: 9.5rem;
    min-height: 2.75rem;
    padding: 0 26px;
    border: 1px solid transparent;
    border-radius: var(--radius-pill);
    font-size: var(--text-sm);
    font-weight: 700;
    cursor: pointer;
    transition:
        background-color var(--duration-base) var(--ease-out),
        border-color var(--duration-base) var(--ease-out),
        color var(--duration-base) var(--ease-out),
        box-shadow var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.follow-button:focus-visible {
    outline: 2px solid var(--primary);
    outline-offset: 3px;
}

.follow-button.is-none {
    background-color: var(--primary);
    box-shadow: var(--shadow-primary);
    color: var(--surface);
}

.follow-button.is-none:hover:not(:disabled) {
    background-color: var(--primary-hover);
    box-shadow: var(--shadow-primary-lift);
    transform: translateY(-2px);
}

.follow-button.is-pending {
    background-color: var(--badge-purple-bg);
    border-color: color-mix(in srgb, var(--primary) 18%, transparent);
    box-shadow: none;
    color: var(--primary);
}

.follow-button.is-accepted {
    background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
    box-shadow: none;
    color: var(--text);
}

.follow-button.is-pending:not(:disabled):is(:hover, :focus-visible),
.follow-button.is-accepted:not(:disabled):is(:hover, :focus-visible) {
    background-color: var(--danger-wash);
    border-color: var(--danger-border);
    color: var(--danger);
}

.follow-button:disabled {
    cursor: progress;
}

.follow-button.is-busy {
    opacity: 0.75;
}

.follow-labels {
    display: grid;
    place-items: center;
}

.follow-rest,
.follow-intent {
    grid-area: 1 / 1;
    white-space: nowrap;
    transition:
        opacity var(--duration-fast) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.follow-intent {
    opacity: 0;
    transform: translateY(40%);
}

.follow-button:not(:disabled):is(:hover, :focus-visible) .follow-intent {
    opacity: 1;
    transform: none;
}

.follow-button:not(:disabled):is(:hover, :focus-visible) .follow-labels:has(.follow-intent) .follow-rest {
    opacity: 0;
    transform: translateY(-40%);
}

@media (hover: none) {
    .follow-labels:has(.follow-intent) .follow-rest {
        opacity: 0;
    }

    .follow-intent {
        opacity: 1;
        transform: none;
    }
}

.label-enter-active,
.label-leave-active {
    transition:
        opacity 140ms var(--ease-out),
        transform 140ms var(--ease-out);
}

.label-enter-from {
    opacity: 0;
    transform: translateY(35%);
}

.label-leave-to {
    opacity: 0;
    transform: translateY(-35%);
}

.action-error {
    margin-top: var(--space-4);
}

.fade-enter-active,
.fade-leave-active {
    transition: opacity var(--duration-base) var(--ease-out);
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}

.stats {
    display: flex;
    flex-wrap: wrap;
    gap: 0 44px;
    margin: 24px 0 0;
    padding: 14px 12px;
    list-style: none;
    border-block: 1px solid var(--border);
}

.stat {
    display: inline-flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0;
    padding: 4px 10px;
    margin-inline: -10px;
    line-height: 1.3;
}

.stat-value {
    display: inline-grid;
    color: var(--text);
    font-family: var(--font-display);
    font-size: 1.25rem;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
}

.stat-value > span {
    grid-area: 1 / 1;
}

.stat-label {
    color: var(--text-muted);
    font-size: var(--text-meta);
    font-weight: 500;
}

.stat-button {
    background: none;
    border: 0;
    border-radius: 14px;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
}

.stat-button:hover {
    background-color: var(--badge-purple-bg);
}

.stat-button:hover .stat-label {
    color: var(--primary);
}

.count-enter-active,
.count-leave-active {
    transition:
        opacity var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.count-enter-active {
    animation: count-flash 900ms var(--ease-out);
}

.count-enter-from {
    opacity: 0;
    transform: translateY(60%);
}

.count-leave-to {
    opacity: 0;
    transform: translateY(-60%);
}

@keyframes count-flash {
    0%,
    45% {
        color: var(--primary);
    }
}

.about {
    margin-top: 20px;
}

.about-me {
    margin: 0 0 12px;
    font-size: var(--text-body);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
}

.details {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-6);
    margin: 0;
    font-size: var(--text-sm);
}

.details div {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
}

.details dt {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-muted);
}

.details dt .nav-icon {
    width: 0.9rem;
    height: 0.9rem;
    color: var(--text-placeholder);
}

.details dd {
    margin: 0;
    font-weight: 700;
    overflow-wrap: anywhere;
}

.card {
    background-color: var(--surface);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-card);
}

.section {
    margin-top: 36px;
}

.section-title {
    margin: 0 0 14px;
}

.card-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 6px;
    font-size: var(--text-card-title);
    font-weight: 700;
}

.section-count {
    min-width: 1.4rem;
    padding: 0 7px;
    background-color: var(--notification-dot);
    border-radius: var(--radius-pill);
    color: var(--surface);
    font-family: var(--font-sans);
    font-size: 0.75rem;
    font-weight: 700;
    line-height: 1.4rem;
    text-align: center;
}

.requests-card {
    margin-top: 24px;
    padding: 20px 22px 10px;
}

.requests {
    position: relative;
    margin: 0;
    padding: 0;
    list-style: none;
}

.request {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2) var(--space-4);
    padding: 10px 0;
}

.request + .request {
    border-top: 1px solid var(--border-hairline);
}

.request-actions {
    display: flex;
    gap: var(--space-2);
}

.request-list-leave-active {
    position: absolute;
    right: 0;
    left: 0;
    transition:
        opacity var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.request-list-leave-to {
    opacity: 0;
    transform: translateX(16px);
}

.request-list-move {
    transition: transform 320ms var(--ease-out);
}

.collapse-enter-active,
.collapse-leave-active {
    overflow: hidden;
    transition:
        opacity var(--duration-base) var(--ease-out),
        max-height 320ms var(--ease-out),
        margin-top 320ms var(--ease-out),
        padding 320ms var(--ease-out);
    max-height: 40rem;
}

.collapse-enter-from,
.collapse-leave-to {
    max-height: 0;
    margin-top: 0;
    padding-block: 0;
    opacity: 0;
}

.person {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    color: var(--text);
    text-decoration: none;
}

.person-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: 1.3;
}

.person-name,
.person-detail {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.person-name {
    font-size: var(--text-sm);
    font-weight: 700;
    transition: color var(--duration-fast) var(--ease-out);
}

.person-detail {
    color: var(--text-muted);
    font-size: var(--text-meta);
}

a.person:hover .person-name {
    color: var(--primary);
}

.posts {
    display: flex;
    flex-direction: column;
    gap: 14px;
    margin: 0;
    padding: 0;
    list-style: none;
}

.post {
    padding: 18px 22px 20px;
    background-color: var(--surface);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-card);
}

.post-meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-meta);
}

.post-time {
    color: inherit;
    text-decoration: none;
    transition: color var(--duration-fast) var(--ease-out);
}

.post-time:hover {
    color: var(--primary);
}

.post-content {
    margin: 8px 0 0;
    font-size: var(--text-body);
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
}

.post-image {
    display: block;
    width: 100%;
    max-height: 26rem;
    margin-top: 14px;
    object-fit: cover;
    background-color: var(--badge-purple-bg);
    border-radius: 16px;
}

.locked,
.empty-state {
    display: grid;
    justify-items: center;
    gap: var(--space-2);
    padding: 56px var(--space-5);
    text-align: center;
}

.locked {
    margin-top: 24px;
}

.empty-state {
    background-color: var(--surface);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-card);
}

.locked-icon,
.empty-icon {
    display: grid;
    place-items: center;
    width: 4.5rem;
    height: 4.5rem;
    margin-bottom: var(--space-2);
    background-color: var(--badge-muted-bg);
    border-radius: 50%;
    color: var(--primary);
}

.locked-icon .nav-icon,
.empty-icon .nav-icon {
    width: 1.5rem;
    height: 1.5rem;
}

.locked-title {
    margin: 0;
    font-size: var(--text-card-title);
    font-weight: 700;
}

.locked-text,
.empty-text {
    max-width: 22rem;
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
}

.people-dialog {
    width: min(26rem, calc(100% - 2 * var(--space-4)));
    max-width: none;
    max-height: min(34rem, calc(100% - 2 * var(--space-6)));
    padding: 0;
    overflow: hidden;
    background-color: var(--surface);
    border: 0;
    border-radius: var(--radius-lg);
    box-shadow: 0 30px 60px -24px color-mix(in srgb, var(--primary) 45%, transparent);
    color: var(--text);
}

/* showModal() focuses the dialog itself when nothing inside asks for focus, so no ring around its whole edge. */
.people-dialog:focus-visible {
    outline: none;
}

.people-dialog[open] {
    display: flex;
    flex-direction: column;
    opacity: 1;
    transform: none;
    transition:
        opacity var(--duration-base) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

@starting-style {
    .people-dialog[open] {
        opacity: 0;
        transform: translateY(10px) scale(0.96);
    }
}

.people-dialog::backdrop {
    background-color: color-mix(in srgb, var(--text) 32%, transparent);
    backdrop-filter: blur(2px);
}

.people-dialog[open]::backdrop {
    transition: background-color var(--duration-base) var(--ease-out);
}

@starting-style {
    .people-dialog[open]::backdrop {
        background-color: transparent;
    }
}

.dialog-panel {
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 20px 20px 12px;
}

.dialog-head {
    display: flex;
    align-items: center;
    gap: 14px;
    padding-bottom: 12px;
}

.dialog-tabs {
    flex: 1;
}

.icon-button {
    display: grid;
    flex: none;
    place-items: center;
    width: 2.5rem;
    height: 2.5rem;
    background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
    border: 0;
    border-radius: 50%;
    color: var(--text-muted);
    cursor: pointer;
    transition:
        background-color var(--duration-fast) var(--ease-out),
        color var(--duration-fast) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.icon-button:hover {
    background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
    color: var(--text);
    transform: rotate(90deg);
}

.icon-button .nav-icon {
    width: 1.1rem;
    height: 1.1rem;
}

.dialog-empty {
    padding: var(--space-5) var(--space-2);
    text-align: center;
}

.people {
    margin: 0 -8px;
    padding: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    list-style: none;
}

.person-row {
    min-height: 3.5rem;
    padding: 8px;
    border-radius: 16px;
    transition: background-color var(--duration-fast) var(--ease-out);
}

.person-row .avatar {
    width: 2.75rem;
    height: 2.75rem;
}

.person-row:hover {
    background-color: var(--badge-purple-bg);
}

.person-tag {
    flex: none;
    margin-left: auto;
    padding: 4px 12px;
    background-color: var(--badge-muted-bg);
    border-radius: var(--radius-pill);
    color: var(--badge-muted-fg);
    font-size: var(--text-meta);
    font-weight: 700;
}

.person-chevron {
    flex: none;
    width: 1rem;
    height: 1rem;
    margin-left: auto;
    color: var(--text-placeholder);
    transform: rotate(180deg);
    transition:
        color var(--duration-fast) var(--ease-out),
        transform var(--duration-base) var(--ease-out);
}

.person-row:hover .person-chevron {
    color: var(--primary);
    transform: rotate(180deg) translateX(-3px);
}

@media (max-width: 47.99rem) {
    .profile-page {
        padding: 20px 16px 40px;
    }

    .avatar-large {
        width: 4.25rem;
        height: 4.25rem;
    }

    .avatar-large.avatar-initial {
        font-size: 1.4rem;
    }

    .nickname {
        font-size: 1.4rem;
    }

    .profile-action {
        flex-basis: 100%;
        margin-top: 0;
    }

    .follow-button {
        width: 100%;
    }

    .visibility-control {
        justify-content: space-between;
    }

    .stats {
        justify-content: space-around;
        gap: 0 var(--space-4);
        padding-inline: 0;
    }

    .stat {
        align-items: center;
    }

    .requests-card {
        padding: 16px 16px 8px;
    }

    .post {
        padding: 16px 18px 18px;
    }
}

@media (prefers-reduced-motion: reduce) {
    .people-dialog[open],
    .people-dialog[open]::backdrop,
    .follow-button,
    .follow-rest,
    .follow-intent,
    .pill-button,
    .stat-button,
    .icon-button,
    .person-chevron,
    .swap-enter-active,
    .swap-leave-active,
    .label-enter-active,
    .label-leave-active,
    .count-enter-active,
    .count-leave-active,
    .fade-enter-active,
    .fade-leave-active,
    .collapse-enter-active,
    .collapse-leave-active,
    .request-list-leave-active,
    .request-list-move {
        transition-duration: 1ms;
    }

    .count-enter-active {
        animation: none;
    }

    .follow-button:hover,
    .pill-solid:hover,
    .icon-button:hover {
        transform: none;
    }
}
</style>
