<template>
    <div class="cmdb-form form-int">
        <input class="cmdb-form-input form-int-input" type="text"
            :placeholder="placeholder || $t('Form[\'请输入数字\']')"
            :value="value"
            :maxlength="maxlength"
            :disabled="disabled"
            @blur="handleInput($event)"
            @change="handleChange">
    </div>
</template>

<script>
    export default {
        name: 'cmdb-form-int',
        props: {
            value: {
                default: null,
                validator (val) {
                    return typeof val === 'number' || val === '' || val === null
                }
            },
            disabled: {
                type: Boolean,
                default: false
            },
            maxlength: {
                type: Number,
                default: 11
            },
            placeholder: {
                type: String,
                default: ''
            }
        },
        data () {
            return {
                localValue: null
            }
        },
        watch: {
            value (value) {
                this.localValue = this.value === '' ? null : this.value
            },
            localValue (localValue) {
                if (localValue !== this.value) {
                    this.$emit('input', localValue)
                }
            }
        },
        created () {
            this.localValue = this.value === '' ? null : this.value
        },
        methods: {
            handleInput (event) {
                let value = parseInt(event.target.value.trim())
                if (isNaN(value)) {
                    value = null
                }
                event.target.value = value
                this.localValue = value
            },
            handleChange () {
                this.$emit('on-change', this.localValue)
            }
        }
    }
</script>

<style lang="scss" scoped>
    .form-int-input {
        height: 36px;
        width: 100%;
        padding: 0 10px;
        background-color: #fff;
        border: 1px solid $cmdbBorderColor;
        font-size: 14px;
        outline: none;
        &:focus{
            border-color: $cmdbBorderFocusColor;
        }
    }
</style>
