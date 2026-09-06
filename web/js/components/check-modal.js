// js/components/check-modal.js - Enhanced with threshold explanation
window.CheckModal = {
    props: {
        show: Boolean,
        editing: Object,
        form: Object,
        hosts: Array,
        saving: Boolean
    },
    emits: ['close', 'save'],
    methods: {
        isValidDuration(duration) {
            return window.RavenUtils.isValidDuration(duration);
        },
        formatThreshold(current, max) {
            return window.RavenUtils.formatThreshold(current, max);
        },
        getCheckTypeIcon(checkType) {
            return window.RavenUtils.getCheckTypeIcon(checkType);
        }
    },
    template: `
        <div class="modal-overlay" :class="{ active: show }" @click.self="$emit('close')">
            <div class="modal">
                <div class="modal-header">
                    <h3 class="modal-title">{{ editing ? 'Edit Check' : 'Add Check' }}</h3>
                    <button class="close-btn" @click="$emit('close')">
                        <i class="fas fa-times"></i>
                    </button>
                </div>
                <form @submit.prevent="$emit('save')">
                    <div class="form-group">
                        <label class="form-label">Name *</label>
                        <input 
                            v-model="form.name" 
                            class="form-input" 
                            type="text" 
                            required
                            placeholder="e.g., Ping Check"
                        >
                    </div>
                    <div class="form-group">
                        <label class="form-label">Type *</label>
                        <div style="display: flex; align-items: center; gap: 0.5rem;">
                            <i :class="getCheckTypeIcon(form.type)" style="color: var(--primary-color);"></i>
                            <select v-model="form.type" class="form-input" required style="flex: 1;">
                                <option value="ping">Ping - Network connectivity test</option>
                                <option value="tcp">TCP - Port connectivity test</option>
                                <option value="dns">DNS - Record lookup</option>
                                <option value="http">HTTP - Web service check</option>
                                <option value="https">HTTPS - Secure web service check</option>
                                <option value="ssh">SSH - Service and banner check</option>
                                <option value="nagios">Nagios Plugin - External monitoring script</option>
                                <option value="icinga">Icinga Plugin - External monitoring script</option>
                            </select>
                        </div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Hosts *</label>
                        <select v-model="form.hosts" class="form-input" multiple size="4" required>
                            <option v-for="host in hosts" :key="host.id" :value="host.id">
                                {{ host.display_name || host.name }} ({{ host.ipv4 || host.hostname }})
                            </option>
                        </select>
                        <small style="color: var(--text-muted);">Hold Ctrl/Cmd to select multiple hosts</small>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Check Intervals</label>
                        <small style="color: var(--text-muted); display: block; margin-bottom: 0.5rem;">
                            How often to run checks based on current status (e.g., 5m, 30s, 1h)
                        </small>
                        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
                            <div>
                                <label class="form-label" style="font-size: 0.875rem;">
                                    <i class="fas fa-check" style="color: var(--success-color);"></i> OK
                                </label>
                                <input v-model="form.interval.ok" 
                                       class="form-input" 
                                       placeholder="5m"
                                       :class="{ 'error': form.interval.ok && !isValidDuration(form.interval.ok) }">
                            </div>
                            <div>
                                <label class="form-label" style="font-size: 0.875rem;">
                                    <i class="fas fa-exclamation-triangle" style="color: var(--warning-color);"></i> Warning
                                </label>
                                <input v-model="form.interval.warning" 
                                       class="form-input" 
                                       placeholder="2m"
                                       :class="{ 'error': form.interval.warning && !isValidDuration(form.interval.warning) }">
                            </div>
                            <div>
                                <label class="form-label" style="font-size: 0.875rem;">
                                    <i class="fas fa-times-circle" style="color: var(--danger-color);"></i> Critical
                                </label>
                                <input v-model="form.interval.critical" 
                                       class="form-input" 
                                       placeholder="1m"
                                       :class="{ 'error': form.interval.critical && !isValidDuration(form.interval.critical) }">
                            </div>
                            <div>
                                <label class="form-label" style="font-size: 0.875rem;">
                                    <i class="fas fa-question-circle" style="color: var(--text-muted);"></i> Unknown
                                </label>
                                <input v-model="form.interval.unknown" 
                                       class="form-input" 
                                       placeholder="1m"
                                       :class="{ 'error': form.interval.unknown && !isValidDuration(form.interval.unknown) }">
                            </div>
                        </div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">
                            <i class="fas fa-hourglass-half"></i> Soft Fail Threshold
                        </label>
                        <input 
                            v-model="form.threshold" 
                            class="form-input" 
                            type="number"
                            min="1"
                            max="10"
                            placeholder="3"
                            style="width: 150px;"
                        >
                        <div style="background: var(--light-bg); padding: 1rem; border-radius: 0.5rem; margin-top: 0.5rem; border: 1px solid var(--border-color);">
                            <strong style="color: var(--primary-color);">
                                <i class="fas fa-info-circle"></i> Soft Fail Explanation:
                            </strong>
                            <p style="margin: 0.5rem 0; font-size: 0.875rem; line-height: 1.4;">
                                Number of consecutive failures before the status changes from OK to Warning/Critical.
                                This prevents false alarms from temporary network issues.
                            </p>
                            <div style="font-size: 0.875rem; color: var(--text-muted);">
                                <strong>Example with threshold {{ form.threshold || 3 }}:</strong><br>
                                • Failure 1/{{ form.threshold || 3 }}: Status remains OK (soft fail)<br>
                                • Failure 2/{{ form.threshold || 3 }}: Status remains OK (soft fail)<br>
                                • Failure {{ form.threshold || 3 }}/{{ form.threshold || 3 }}: Status changes to Warning/Critical (hard fail)
                            </div>
                        </div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Timeout</label>
                        <input 
                            v-model="form.timeout" 
                            class="form-input" 
                            type="text"
                            placeholder="30s"
                            style="width: 150px;"
                            :class="{ 'error': form.timeout && !isValidDuration(form.timeout) }"
                        >
                        <small style="color: var(--text-muted);">Maximum time to wait for check completion</small>
                    </div>
                    <div class="form-group">
                        <label class="form-checkbox">
                            <input v-model="form.enabled" type="checkbox">
                            <span>Enabled</span>
                        </label>
                        <small style="color: var(--text-muted); display: block; margin-top: 0.5rem;">
                            Disabled checks will not run and won't generate alerts
                        </small>
                    </div>
                    
                    <!-- Show current check statistics if editing -->
                    <div v-if="editing" class="form-group" style="background: var(--light-bg); padding: 1rem; border-radius: 0.5rem; border: 1px solid var(--border-color);">
                        <h4 style="margin-bottom: 0.5rem; color: var(--text-primary);">
                            <i class="fas fa-chart-line"></i> Check Statistics
                        </h4>
                        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; font-size: 0.875rem;">
                            <div>
                                <strong>Check ID:</strong> {{ editing.id }}
                            </div>
                            <div>
                                <strong>Status:</strong>
                                <span v-if="editing.enabled" class="status-badge status-ok" style="margin-left: 0.5rem; padding: 0.125rem 0.5rem;">
                                    ENABLED
                                </span>
                                <span v-else class="status-badge status-unknown" style="margin-left: 0.5rem; padding: 0.125rem 0.5rem;">
                                    DISABLED
                                </span>
                            </div>
                            <div>
                                <strong>Target Hosts:</strong> {{ (editing.hosts && editing.hosts.length) || 0 }}
                            </div>
                            <div>
                                <strong>Current Threshold:</strong> {{ editing.threshold || 3 }} failures
                            </div>
                        </div>
                    </div>
                    
                    <div class="form-actions">
                        <button type="button" class="btn btn-secondary" @click="$emit('close')">Cancel</button>
                        <button type="submit" class="btn btn-primary" :disabled="saving">
                            {{ saving ? 'Saving...' : 'Save' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    `
};
