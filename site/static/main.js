import { autocomp } from './lib.js';

let TAGS = []
if (!localStorage.tags) {
    fetch("/api/tags")
    .then(response => response.json())
    .then(data => {
        TAGS = data.data;
        localStorage.tags = TAGS.join("|");
    });
} else {
    TAGS = localStorage.tags.split("|");
}

const qInput = document.querySelector("input[data-autocomp-tags]");
if (qInput) {
    autocomp(qInput, {
        onQuery: async (val) => {
            const q = val.trim().toLowerCase();
            return TAGS.filter(s => s.includes(q)).slice(0, 10);
        },

        onSelect: (val) => {
            return val;
        }
    });
}
