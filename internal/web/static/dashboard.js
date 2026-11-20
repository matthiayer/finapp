"use strict";

(() => {
  const range = document.getElementById("range");
  const customRangeFields = document.getElementById("custom-range-fields");
  if (range && customRangeFields) {
    const updateCustomRangeFields = () => {
      const enabled = range.value === "custom";
      customRangeFields.hidden = !enabled;
      customRangeFields.querySelectorAll("input").forEach((input) => {
        input.required = enabled;
      });
    };
    range.addEventListener("change", updateCustomRangeFields);
    updateCustomRangeFields();
  }

  const element = document.getElementById("dashboard-data");
  if (!element || typeof Chart === "undefined") return;

  const data = JSON.parse(element.textContent);
  const number = (value) => value === "" ? null : Number(value);
  const colors = ["#176b4a", "#d27b53", "#5377ba", "#9d71ad", "#bd9e37", "#5296a0"];

  const monthly = document.getElementById("monthly-chart");
  if (monthly) {
    new Chart(monthly, {
      data: {
        labels: data.months,
        datasets: [
          { type: "bar", label: "Income", data: data.income.map(number), backgroundColor: "#69ad83" },
          { type: "bar", label: "Spending", data: data.spending.map(number), backgroundColor: "#dc896d" },
          { type: "line", label: "Saving", data: data.saving.map(number), borderColor: "#4269a8", backgroundColor: "#4269a8", tension: 0.2 }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: "index", intersect: false },
        scales: { y: { beginAtZero: true } }
      }
    });
  }

  const balance = document.getElementById("balance-chart");
  if (balance) {
    const datasets = data.balanceSeries.map((series, index) => ({
      label: series.label,
      data: series.values.map(number),
      borderColor: colors[index % colors.length],
      backgroundColor: colors[index % colors.length],
      tension: 0.15,
      spanGaps: false
    }));
    datasets.push({
      label: "Combined observed balance",
      data: data.balances.map(number),
      borderColor: "#17231d",
      backgroundColor: "#17231d",
      borderDash: [5, 4],
      borderWidth: 3,
      tension: 0.15
    });
    new Chart(balance, {
      type: "line",
      data: { labels: data.balanceMonths, datasets },
      options: { responsive: true, maintainAspectRatio: false, spanGaps: false }
    });
  }

  const spending = document.getElementById("spending-chart");
  if (spending) {
    new Chart(spending, {
      type: "bar",
      data: {
        labels: data.spendingLabels,
        datasets: [{ label: "Spending (€)", data: data.spendingValues.map(number), backgroundColor: "#dc896d" }]
      },
      options: { indexAxis: "y", responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } } }
    });
  }

  const income = document.getElementById("income-chart");
  if (income) {
    new Chart(income, {
      type: "doughnut",
      data: {
        labels: data.incomeLabels,
        datasets: [{ data: data.incomeValues.map(number), backgroundColor: colors }]
      },
      options: { responsive: true, maintainAspectRatio: false }
    });
  }
})();
