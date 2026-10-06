<script setup>
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import NavIcon from "../components/NavIcon.vue";

const router = useRouter();
const auth = useAuthStore();

const email = ref("");
const password = ref("");
const firstName = ref("");
const lastName = ref("");
const dateOfBirth = ref("");
const nickname = ref("");
const aboutMe = ref("");
const avatar = ref(null);

const showPassword = ref(false);

// Mirrors personNamePattern in backend/pkg/auth/routes.go (the nickname's character class), which is what actually enforces it.
const NAME_PATTERN = /^[A-Za-z0-9._-]+$/;

const firstNameInvalid = computed(() => firstName.value !== "" && !NAME_PATTERN.test(firstName.value));
const lastNameInvalid = computed(() => lastName.value !== "" && !NAME_PATTERN.test(lastName.value));

// Mirror visibleASCIIPattern and aboutMePattern in backend/pkg/auth/routes.go, which is what actually enforces them.
const VISIBLE_ASCII_PATTERN = /^[!-~]+$/;
const ABOUT_ME_PATTERN = /^[ -~\r\n]*$/;
const ABOUT_ME_MESSAGE = "About me may only contain English letters, numbers, symbols, and spaces.";

const emailInvalid = computed(() => email.value !== "" && !VISIBLE_ASCII_PATTERN.test(email.value));
const passwordInvalid = computed(() => password.value !== "" && !VISIBLE_ASCII_PATTERN.test(password.value));
const aboutMeInvalid = computed(() => !ABOUT_ME_PATTERN.test(aboutMe.value));

// A textarea has no pattern attribute, so a custom validity blocks the submit the way the inputs' patterns do.
const aboutMeInput = ref(null);
watch(aboutMeInvalid, (invalid) => {
    aboutMeInput.value?.setCustomValidity(invalid ? ABOUT_ME_MESSAGE : "");
});

function handleAvatar(event) {
    avatar.value = event.target.files[0] || null;
}

async function handleSubmit() {
    const formData = new FormData();

    formData.append("email", email.value);
    formData.append("password", password.value);
    formData.append("first_name", firstName.value);
    formData.append("last_name", lastName.value);
    formData.append("date_of_birth", dateOfBirth.value);

    formData.append("nickname", nickname.value);

    if (aboutMe.value) {
        formData.append("about_me", aboutMe.value);
    }

    if (avatar.value) {
        formData.append("avatar", avatar.value);
    }

    try {
        await auth.register(formData);
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
        <h1 class="visually-hidden">Create account</h1>

        <div class="field-group">
            <div class="field-row">
                <div class="field">
                    <label
                        for="register-first-name"
                        class="visually-hidden"
                    >First name</label>
                    <NavIcon
                        name="profile"
                        class="field-icon"
                    />
                    <input
                        id="register-first-name"
                        v-model="firstName"
                        type="text"
                        placeholder="First name"
                        autocomplete="given-name"
                        maxlength="50"
                        pattern="[A-Za-z0-9._\-]+"
                        :aria-invalid="firstNameInvalid"
                        aria-describedby="register-name-hint"
                        required
                    />
                </div>

                <div class="field">
                    <label
                        for="register-last-name"
                        class="visually-hidden"
                    >Last name</label>
                    <NavIcon
                        name="profile"
                        class="field-icon"
                    />
                    <input
                        id="register-last-name"
                        v-model="lastName"
                        type="text"
                        placeholder="Last name"
                        autocomplete="family-name"
                        maxlength="50"
                        pattern="[A-Za-z0-9._\-]+"
                        :aria-invalid="lastNameInvalid"
                        aria-describedby="register-name-hint"
                        required
                    />
                </div>
            </div>
            <p
                id="register-name-hint"
                class="hint"
                :class="{ 'hint-invalid': firstNameInvalid || lastNameInvalid }"
            >
                Names use English letters, digits, dots, hyphens or underscores, with no spaces.
            </p>
        </div>

        <div class="field-group">
            <div class="field">
                <label
                    for="register-email"
                    class="visually-hidden"
                >Email</label>
                <NavIcon
                    name="mail"
                    class="field-icon"
                />
                <input
                    id="register-email"
                    v-model="email"
                    type="email"
                    placeholder="Email"
                    autocomplete="email"
                    pattern="[!-~]+"
                    :aria-invalid="emailInvalid"
                    aria-describedby="register-email-hint"
                    required
                />
            </div>
            <p
                id="register-email-hint"
                class="hint"
                :class="{ 'hint-invalid': emailInvalid }"
            >
                Use English letters, numbers, and standard symbols.
            </p>
        </div>

        <div class="field-group">
            <div class="field">
                <label
                    for="register-nickname"
                    class="visually-hidden"
                >Nickname</label>
                <NavIcon
                    name="at"
                    class="field-icon"
                />
                <input
                    id="register-nickname"
                    v-model="nickname"
                    type="text"
                    placeholder="Nickname"
                    autocomplete="username"
                    autocapitalize="off"
                    spellcheck="false"
                    minlength="3"
                    maxlength="30"
                    pattern="[A-Za-z0-9._\-]{3,30}"
                    aria-describedby="register-nickname-hint"
                    required
                />
            </div>
            <p
                id="register-nickname-hint"
                class="hint"
            >
                Use 3 to 30 letters, digits, dots, hyphens or underscores.
            </p>
        </div>

        <div class="field-group">
            <div class="field">
                <label
                    for="register-password"
                    class="visually-hidden"
                >Password</label>
                <NavIcon
                    name="lock"
                    class="field-icon"
                />
                <input
                    id="register-password"
                    v-model="password"
                    :type="showPassword ? 'text' : 'password'"
                    placeholder="Password"
                    autocomplete="new-password"
                    pattern="[!-~]+"
                    :aria-invalid="passwordInvalid"
                    aria-describedby="register-password-hint"
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
            <p
                id="register-password-hint"
                class="hint"
                :class="{ 'hint-invalid': passwordInvalid }"
            >
                Use 8 to 72 characters: English letters, numbers, and symbols (no spaces or emoji).
            </p>
        </div>

        <div class="field">
            <NavIcon
                name="calendar"
                class="field-icon"
            />
            <label
                for="register-date-of-birth"
                class="field-inline-label"
            >Date of birth</label>
            <input
                id="register-date-of-birth"
                v-model="dateOfBirth"
                type="date"
                required
            />
        </div>

        <section
            class="optional"
            aria-labelledby="register-optional"
        >
            <h2
                id="register-optional"
                class="section-label"
            >
                Optional
            </h2>

            <div class="field field-file">
                <NavIcon
                    name="image"
                    class="field-icon"
                />
                <label
                    for="register-avatar"
                    class="field-inline-label"
                >Avatar</label>
                <input
                    id="register-avatar"
                    type="file"
                    accept="image/jpeg,image/png,image/gif"
                    @change="handleAvatar"
                />
            </div>

            <div class="field-group">
                <div class="field field-textarea">
                    <label
                        for="register-about-me"
                        class="visually-hidden"
                    >About me</label>
                    <NavIcon
                        name="text"
                        class="field-icon"
                    />
                    <textarea
                        id="register-about-me"
                        ref="aboutMeInput"
                        v-model="aboutMe"
                        rows="3"
                        maxlength="500"
                        placeholder="About me"
                        :aria-invalid="aboutMeInvalid"
                        aria-describedby="register-about-me-hint"
                    ></textarea>
                </div>
                <p
                    id="register-about-me-hint"
                    class="hint"
                    :class="{ 'hint-invalid': aboutMeInvalid }"
                >
                    Use English letters, numbers, symbols, and spaces.
                </p>
            </div>
        </section>

        <Transition name="error-reveal">
            <p
                v-if="auth.error"
                class="error"
                role="alert"
            >
                {{ auth.error }}
            </p>
        </Transition>

        <button
            type="submit"
            class="primary-button"
            :disabled="auth.loading"
        >
            {{
                auth.loading
                    ? "Creating account..."
                    : "Create account"
            }}
        </button>
    </form>
</template>

<style scoped src="../styles/auth.css"></style>
