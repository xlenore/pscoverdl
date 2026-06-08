const bridge = () => window.go?.gui?.App;
let _selected = "duckstation";
let currentConfig = null;

const $ = (id) => document.getElementById(id);

const emulatorLabels = {
	duckstation: "DuckStation",
	pcsx2: "PCSX2",
};

function setSelected(emulator) {
	_selected = emulator;
	const label = emulatorLabels[emulator] || emulator;
	const title = $("panel-title");
	const pill = $("emulator-pill");
	if (title) title.textContent = label;
	if (pill) pill.textContent = label;
	document.querySelectorAll(".nav-button").forEach((button) => {
		button.classList.toggle("active", button.dataset.emulator === emulator);
	});
	document.querySelectorAll(".form-page").forEach((page) => {
		page.hidden = page.dataset.page !== emulator;
	});
}

function coverType(emulator) {
	const checked = document.querySelector(
		`input[name="${emulator}-cover-type"]:checked`,
	);
	return Number(checked ? checked.value : 0);
}

function setCoverType(emulator, value) {
	const input = document.querySelector(
		`input[name="${emulator}-cover-type"][value="${value}"]`,
	);
	if (input) input.checked = true;
}

function readEmulator(emulator) {
	return {
		coverDirectory: $(`${emulator}-cover-directory`).value,
		gameCache: $(`${emulator}-game-cache`).value,
		coverType: coverType(emulator),
		useSSL: $(`${emulator}-use-ssl`).checked,
		fallback: $(`${emulator}-fallback`).checked,
	};
}

function writeEmulator(emulator, cfg) {
	$(`${emulator}-cover-directory`).value = cfg.coverDirectory || "";
	$(`${emulator}-game-cache`).value = cfg.gameCache || "";
	setCoverType(emulator, cfg.coverType || 0);
	$(`${emulator}-use-ssl`).checked = cfg.useSSL !== false;
	$(`${emulator}-fallback`).checked = Boolean(cfg.fallback);
}

function readConfig() {
	return {
		duckstation: readEmulator("duckstation"),
		pcsx2: readEmulator("pcsx2"),
	};
}

function cliConfig(emulator) {
	const cfg = readEmulator(emulator);
	return {
		emulator,
		coverDir: cfg.coverDirectory,
		gameListPath: cfg.gameCache,
		coverType: cfg.coverType === 1 ? "3d" : "default",
		useHTTP: !cfg.useSSL,
		fallback: cfg.fallback,
		workers: 4,
	};
}

function appendOutput(line) {
	const output = $("output");
	if (!output || !line) return;
	output.textContent = output.textContent
		? `${output.textContent}\n${line}`
		: line;
	output.scrollTop = output.scrollHeight;
}

function setBusy(busy) {
	document.querySelectorAll("button").forEach((button) => {
		button.disabled = busy;
	});
}

async function loadConfig() {
	const api = bridge();
	if (!api) return;
	currentConfig = await api.LoadConfig();
	writeEmulator("duckstation", currentConfig.duckstation || {});
	writeEmulator("pcsx2", currentConfig.pcsx2 || {});
}

async function saveConfig() {
	const api = bridge();
	if (!api) return "";
	return api.SaveConfig(readConfig());
}

async function browse(emulator, kind) {
	const api = bridge();
	if (!api) return;
	const path =
		kind === "cache"
			? await api.SelectGameCache()
			: await api.SelectCoverDirectory();
	if (!path) return;
	const id =
		kind === "cache" ? `${emulator}-game-cache` : `${emulator}-cover-directory`;
	$(id).value = path;
}

async function detectPaths(emulator) {
	const api = bridge();
	if (!api) return;
	const cfg = await api.DetectDefaults(emulator);
	writeEmulator(emulator, { ...readEmulator(emulator), ...cfg });
}

async function startDownload(emulator) {
	const api = bridge();
	if (!api) return;
	const output = $("output");
	output.textContent = "";
	appendOutput("Running pscoverdl...");
	setBusy(true);
	const result = await api.StartDownload(cliConfig(emulator));
	await saveConfig();
	setBusy(false);
	if (result.error) appendOutput(result.error);
	if (!output.textContent.trim()) {
		output.textContent = [result.command, result.output]
			.filter(Boolean)
			.join("\n");
	}
}

function wireEvents() {
	document.querySelectorAll(".nav-button").forEach((button) => {
		button.addEventListener("click", () =>
			setSelected(button.dataset.emulator),
		);
	});
	document.querySelectorAll("[data-browse]").forEach((button) => {
		button.addEventListener("click", () =>
			browse(button.dataset.emulator, button.dataset.browse),
		);
	});
	document.querySelectorAll("[data-start]").forEach((button) => {
		button.addEventListener("click", (event) => {
			event.preventDefault();
			startDownload(button.dataset.start);
		});
	});
	document.querySelectorAll("[data-detect]").forEach((button) => {
		button.addEventListener("click", () => detectPaths(button.dataset.detect));
	});
	$("cover-form").addEventListener("submit", (event) => event.preventDefault());
}

wireEvents();
if (window.runtime?.EventsOn) {
	window.runtime.EventsOn("download-progress", appendOutput);
}
setSelected("duckstation");
loadConfig();
