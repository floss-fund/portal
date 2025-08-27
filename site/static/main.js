import { autocomp } from "./lib.js";

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
const isTags = document.querySelector(".search input[name=field][value=tags]");
if (qInput) {
    autocomp(qInput, {
        onQuery: async (val) => {
            if (!isTags.checked) {
                return [];
            }
            const q = val.trim().toLowerCase();
            return TAGS.filter(s => s.includes(q)).slice(0, 10);
        },

        onSelect: (val) => {
            return val;
        }
    });
}

// Listen for ~ key and focus on the search bar.
document.addEventListener("keydown", function(event) {
  if (event.key === "`") {
    event.preventDefault();
    const q = document.querySelector("form.search input[name=q]");
    q.focus();
    q.select();
  }
});

(() => {
    const params = new URLSearchParams(location.search);

    // Set initial values
    ["order_by", "order"].forEach(param => {
      if (params.has(param)) {
        document.querySelector(`select[name="${param}"] option.${params.get(param)}`).selected = true;
      }
    });

    // Handle changes
    document.querySelectorAll(".order select").forEach(e =>
      e.onchange = () => {
        params.set("page", 1);
        params.set(e.name, e.options[e.selectedIndex].className);
        location.search = params.toString();
      }
    );

    // Unverified links.
    document.querySelectorAll('a[data-unverified]').forEach(el => {
        el.addEventListener('click', function(ev) {
            ev.preventDefault();
            const dialog = document.querySelector('dialog.unverified-link').cloneNode(true);
            document.body.appendChild(dialog);

            const link = dialog.querySelector('.continue-link');
            link.href = el.href;
            link.addEventListener('click', function() {
                dialog.close();
                dialog.remove();
              });

              dialog.querySelector('.btn-cancel').addEventListener('click', function() {
                dialog.close();
                dialog.remove();
            });

            dialog.showModal();
        });
    });
})();