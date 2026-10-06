<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import NavIcon from "../components/NavIcon.vue";

const router = useRouter();
const authStore = useAuthStore();

const identifier = ref("");
const password = ref("");

const showPassword = ref(false);

async function handleSubmit() {
    try {
        await authStore.login(
            identifier.value,
            password.value,
        );

        router.push("/");
    } catch {
    }
}
</script>

<template>
    <form
        class="auth-form"
        @submit.prevent="handleSubmit"
    >
        <h1 class="visually-hidden">Login</h1>

        <div class="field">
            <label
                for="login-identifier"
                class="visually-hidden"
            >Email or nickname</label>
            <NavIcon
                name="profile"
                class="field-icon"
            />
            <!-- type="text": a nickname isn't an email address, and an email input would refuse to submit one. -->
            <input
                id="login-identifier"
                v-model="identifier"
                type="text"
                placeholder="Email or nickname"
                autocomplete="username"
                autocapitalize="off"
                spellcheck="false"
                required
            />
        </div>

        <div class="field">
            <label
                for="login-password"
                class="visually-hidden"
            >Password</label>
            <NavIcon
                name="lock"
                class="field-icon"
            />
            <input
                id="login-password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="Password"
                autocomplete="current-password"
                required
            />
            <button
                type="button"
                class="field-toggle"
                :aria-label="showPassword ? 'Hide password' : 'Show password'"
                :aria-pressed="showPassword"
                @click="showPassword = !showPassword"
            >
                <NavIcon :name="showPassword ? 'eye-off' : 'eye'" />
            </button>
        </div>

        <Transition name="error-reveal">
            <p
                v-if="authStore.notice"
                class="notice"
                role="status"
            >
                {{ authStore.notice }}
            </p>
        </Transition>

        <Transition name="error-reveal">
            <p
                v-if="authStore.error"
                class="error"
                role="alert"
            >
                {{ authStore.error }}
            </p>
        </Transition>

        <button
            type="submit"
            class="primary-button"
            :disabled="authStore.loading"
        >
            {{
                authStore.loading
                    ? "Logging in..."
                    : "Get started"
            }}
        </button>
    </form>
</template>

<style scoped src="../styles/auth.css"></style>
