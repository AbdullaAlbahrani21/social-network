import { defineStore } from "pinia";
import { api } from "../api";
import { useChatStore } from "./chat";
import { useNotificationStore } from "./notifications";
import { useTypingStore } from "./typing";
import {
    CLOSE_REASON_LOGGED_IN_ELSEWHERE,
    useWebSocketStore,
} from "./websocket";

const signedInElsewhereMessage =
    "You were logged out because your account was signed in from another location.";

function clearUserSession() {
    useWebSocketStore().disconnect();
    useChatStore().reset();
    useNotificationStore().reset();
    useTypingStore().reset();
}

let sessionCheck = null;

export const useAuthStore = defineStore("auth", {
    state: () => ({
        user: null,
        loading: false,
        error: null,
        notice: null,
    }),

    getters: {
        isLoggedIn: (state) => state.user !== null,
    },

    actions: {
        async register(formData) {
            this.loading = true;
            this.error = null;

            try {
                this.setUser(
                    await api.upload(
                        "/api/register",
                        formData,
                    ),
                );
            } catch (error) {
                this.error = error.message;
                throw error;
            } finally {
                this.loading = false;
            }
        },

        async login(email, password) {
            this.loading = true;
            this.error = null;
            this.notice = null;

            try {
                this.setUser(
                    await api.post("/api/login", {
                        email,
                        password,
                    }),
                );
            } catch (error) {
                this.error = error.message;
                throw error;
            } finally {
                this.loading = false;
            }
        },

        async fetchMe() {
            this.loading = true;

            // /api/session, not /api/me: a 401 here is printed to the console by the browser's network stack, where no catch can reach it.
            try {
                const { user } = await api.get("/api/session");
                if (user) this.setUser(user);
                else this.user = null;
            } catch {
                this.user = null;
            } finally {
                this.loading = false;
            }
        },

        // The first call sends the request and every later one gets it back, so the router guard can await it on every navigation without sending more.
        checkSession() {
            if (!sessionCheck) {
                sessionCheck = this.fetchMe();
            }

            return sessionCheck;
        },

        setUser(user) {
            if (this.user?.id !== user.id) {
                clearUserSession();
            }

            this.user = user;
        },

        // The socket closes before the request, and the user is cleared whether or not it succeeds; the error is kept rather than rethrown, since logout is bound straight to a click.
        async logout() {
            useWebSocketStore().disconnect();

            try {
                await api.post("/api/logout");
                this.error = null;
            } catch (error) {
                this.error = error.message;
            } finally {
                clearUserSession();
                this.user = null;
            }
        },

        endSession(reason) {
            this.notice =
                reason === CLOSE_REASON_LOGGED_IN_ELSEWHERE
                    ? signedInElsewhereMessage
                    : null;

            this.error = null;

            clearUserSession();
            this.user = null;
        },
    },
});
