package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JSONStore implementats Storage interface - for JSON file storage
type jsonStore struct {
	configPath string
	filePath   string
	mu         sync.RWMutex
	defaults   map[string]string // allows reusing defaults without querying for config
}

type expensesFileData struct {
	Expenses []Expense `json:"expenses"`
}

func InitializeJsonStore(baseConfig SystemConfig) (*jsonStore, error) {
	configPath := filepath.Join(baseConfig.StorageURL, "config.json")
	filePath := filepath.Join(baseConfig.StorageURL, "expenses.json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %v", err)
	}

	// create expenses file if it doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		initialData := expensesFileData{Expenses: []Expense{}}
		data, err := json.Marshal(initialData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal initial data: %v", err)
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to create storage file: %v", err)
		}
		log.Println("Created expense storage file")
	} else {
		log.Println("Found existing expense storage file")
	}

	// create config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		initialConfig := Config{}
		initialConfig.SetBaseConfig()
		data, err := json.Marshal(initialConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal initial config: %v", err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to create config file: %v", err)
		}
		log.Println("Created expense storage config")
	} else {
		log.Println("Found existing expense storage config")
	}

	return &jsonStore{
		configPath: configPath,
		filePath:   filePath,
		defaults:   map[string]string{},
	}, nil
}

// primitive methods

func (s *jsonStore) readExpensesFile(path string) (*expensesFileData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data expensesFileData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	log.Println("Read expenses file")
	return &data, nil
}

func (s *jsonStore) writeExpensesFile(path string, data *expensesFileData) error {
	content, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	log.Println("Wrote expenses file")
	return os.WriteFile(path, content, 0644)
}

func (s *jsonStore) readConfigFile(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data Config
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	log.Println("Read config file")
	return &data, nil
}

func (s *jsonStore) writeConfigFile(path string, data *Config) error {
	content, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	log.Println("Wrote config file")
	return os.WriteFile(path, content, 0644)
}

// ------------------------------------------------------------
// JSONStore interface methods
// ------------------------------------------------------------

func (s *jsonStore) Close() error {
	return nil
}

func (s *jsonStore) GetConfig() (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readConfigFile(s.configPath)
}

// Basic Config Updates

func (s *jsonStore) GetCategories() ([]string, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return config.Categories, nil
}

func (s *jsonStore) UpdateCategories(categories []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	data.Categories = categories
	return s.writeConfigFile(s.configPath, data)
}

func (s *jsonStore) GetCurrency() (string, error) {
	config, err := s.GetConfig()
	if err != nil {
		return "", err
	}
	return config.Currency, nil
}

func (s *jsonStore) UpdateCurrency(currency string) error {
	if !slices.Contains(SupportedCurrencies, currency) {
		return fmt.Errorf("invalid currency: %s", currency)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	data.Currency = currency
	s.defaults["currency"] = currency
	return s.writeConfigFile(s.configPath, data)
}

func (s *jsonStore) GetStartDate() (int, error) {
	config, err := s.GetConfig()
	if err != nil {
		return 0, err
	}
	return config.StartDate, nil
}

func (s *jsonStore) UpdateStartDate(startDate int) error {
	if startDate < 1 || startDate > 31 {
		return fmt.Errorf("invalid start date: %d", startDate)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	data.StartDate = startDate
	s.defaults["start_date"] = fmt.Sprintf("%d", startDate)
	return s.writeConfigFile(s.configPath, data)
}

func (s *jsonStore) GetAutoCarryForward() (bool, error) {
	config, err := s.GetConfig()
	if err != nil {
		return false, err
	}
	return config.AutoCarryForward, nil
}

func (s *jsonStore) UpdateAutoCarryForward(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	data.AutoCarryForward = enabled
	return s.writeConfigFile(s.configPath, data)
}

func (s *jsonStore) GetRecurringExpenses() ([]RecurringExpense, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return config.RecurringExpenses, nil
}

func (s *jsonStore) GetRecurringExpense(id string) (RecurringExpense, error) {
	recurringExpenses, err := s.GetRecurringExpenses()
	if err != nil {
		return RecurringExpense{}, err
	}
	for _, r := range recurringExpenses {
		if r.ID == id {
			return r, nil
		}
	}
	return RecurringExpense{}, fmt.Errorf("recurring expense with ID %s not found", id)
}

func (s *jsonStore) AddRecurringExpense(recurringExpense RecurringExpense) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	if recurringExpense.ID == "" {
		recurringExpense.ID = uuid.New().String()
	}
	if recurringExpense.Currency == "" {
		recurringExpense.Currency = s.defaults["currency"]
	}
	config.RecurringExpenses = append(config.RecurringExpenses, recurringExpense)
	if err := s.writeConfigFile(s.configPath, config); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	expensesToAdd := generateExpensesFromRecurring(recurringExpense, false)
	return s.AddMultipleExpenses(expensesToAdd)
}

func (s *jsonStore) RemoveRecurringExpense(id string, removeAll bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	var found bool
	var updatedRecurringExpenses []RecurringExpense
	for _, r := range config.RecurringExpenses {
		if r.ID == id {
			found = true
		} else {
			updatedRecurringExpenses = append(updatedRecurringExpenses, r)
		}
	}
	if !found {
		return fmt.Errorf("recurring expense with ID %s not found", id)
	}
	config.RecurringExpenses = updatedRecurringExpenses
	expensesData, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	var updatedExpenses []Expense
	today := time.Now()
	for _, exp := range expensesData.Expenses {
		if exp.RecurringID != id {
			updatedExpenses = append(updatedExpenses, exp)
			continue
		}
		if !removeAll && !exp.Date.After(today) {
			updatedExpenses = append(updatedExpenses, exp)
		}
	}
	expensesData.Expenses = updatedExpenses
	if err := s.writeExpensesFile(s.filePath, expensesData); err != nil {
		return err
	}
	return s.writeConfigFile(s.configPath, config)
}

func (s *jsonStore) UpdateRecurringExpense(id string, recurringExpense RecurringExpense, updateAll bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readConfigFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	var found bool
	for i, r := range config.RecurringExpenses {
		if r.ID == id {
			recurringExpense.ID = id // Ensure ID is preserved
			if recurringExpense.Currency == "" {
				recurringExpense.Currency = s.defaults["currency"]
			}
			config.RecurringExpenses[i] = recurringExpense
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("recurring expense with ID %s not found", id)
	}
	expensesData, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	var remainingExpenses []Expense
	today := time.Now()
	for _, exp := range expensesData.Expenses {
		if exp.RecurringID != id {
			remainingExpenses = append(remainingExpenses, exp)
			continue
		}
		if !updateAll && !exp.Date.After(today) {
			remainingExpenses = append(remainingExpenses, exp)
		}
	}
	expensesData.Expenses = remainingExpenses
	expensesToAdd := generateExpensesFromRecurring(recurringExpense, !updateAll)
	expensesData.Expenses = append(expensesData.Expenses, expensesToAdd...)
	if err := s.writeExpensesFile(s.filePath, expensesData); err != nil {
		return err
	}
	return s.writeConfigFile(s.configPath, config)
}

// Expenses

func (s *jsonStore) GetAllExpenses() ([]Expense, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage file: %v", err)
	}
	
	// Check if auto-carry-forward is enabled and generate opening balances if needed
	config, err := s.readConfigFile(s.configPath)
	if err != nil {
		return data.Expenses, nil // Return expenses even if config read fails
	}
	
	if config.AutoCarryForward {
		updatedExpenses, err := s.ensureOpeningBalances(data.Expenses, config)
		if err == nil && len(updatedExpenses) > len(data.Expenses) {
			// Save the updated expenses with new opening balances
			data.Expenses = updatedExpenses
			s.writeExpensesFile(s.filePath, data)
		}
	}
	
	return data.Expenses, nil
}

// ensureOpeningBalances checks all months with expenses and creates opening balance
// expenses for months that don't have one yet (when auto-carry-forward is enabled)
func (s *jsonStore) ensureOpeningBalances(expenses []Expense, config *Config) ([]Expense, error) {
	if len(expenses) == 0 {
		return expenses, nil
	}
	
	// Group expenses by month (year-month)
	monthMap := make(map[string][]Expense)
	for _, exp := range expenses {
		yearMonth := fmt.Sprintf("%d-%02d", exp.Date.Year(), exp.Date.Month())
		monthMap[yearMonth] = append(monthMap[yearMonth], exp)
	}
	
	// Sort months to process them in order
	var months []string
	for ym := range monthMap {
		months = append(months, ym)
	}
	
	// Find the earliest month
	if len(months) == 0 {
		return expenses, nil
	}
	
	// Check each month (except the earliest) for opening balance
	newExpenses := make([]Expense, 0)
	currency := config.Currency
	if currency == "" {
		currency = "usd"
	}
	
	// Use "Income" as the default category for opening balance
	// This should exist in default categories
	openingBalanceCategory := "Income"
	
	for yearMonth, monthExps := range monthMap {
		// Check if this month already has an opening balance
		hasOpeningBalance := false
		for _, exp := range monthExps {
			if exp.Name == "Opening Balance (Carried Forward)" {
				hasOpeningBalance = true
				break
			}
		}
		
		if hasOpeningBalance {
			continue
		}
		
		// Parse year and month
		var year, month int
		fmt.Sscanf(yearMonth, "%d-%d", &year, &month)
		
		// Calculate previous month
		prevYear, prevMonth := year, month-1
		if prevMonth == 0 {
			prevYear--
			prevMonth = 12
		}
		prevYearMonth := fmt.Sprintf("%d-%02d", prevYear, prevMonth)
		
		// Get previous month expenses
		prevMonthExps, exists := monthMap[prevYearMonth]
		if !exists {
			continue // No previous month data, skip
		}
		
		// Calculate previous month balance
		prevIncome := 0.0
		prevExpenses := 0.0
		for _, exp := range prevMonthExps {
			if exp.Amount > 0 {
				prevIncome += exp.Amount
			} else {
				prevExpenses += -exp.Amount
			}
		}
		prevBalance := prevIncome - prevExpenses
		
		// Only create opening balance if there's a positive balance to carry forward
		if prevBalance > 0 {
			// Create opening balance expense for the first day of current month
			openingDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			openingExp := GenerateOpeningBalanceExpense(prevBalance, currency, openingDate, openingBalanceCategory)
			newExpenses = append(newExpenses, openingExp)
		}
	}
	
	// Add new opening balance expenses to the list
	if len(newExpenses) > 0 {
		expenses = append(expenses, newExpenses...)
	}
	
	return expenses, nil
}

func (s *jsonStore) GetExpense(id string) (Expense, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return Expense{}, fmt.Errorf("failed to read storage file: %v", err)
	}
	for i, exp := range data.Expenses {
		if exp.ID == id {
			log.Printf("Retrieved expense with ID %s\n", id)
			return data.Expenses[i], nil
		}
	}
	return Expense{}, fmt.Errorf("expense with ID %s not found", id)
}

func (s *jsonStore) AddExpense(expense Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	if expense.ID == "" {
		expense.ID = uuid.New().String()
	}
	if expense.Currency == "" {
		expense.Currency = s.defaults["currency"]
	}
	if expense.Date.IsZero() {
		expense.Date = time.Now()
	}
	data.Expenses = append(data.Expenses, expense)
	log.Printf("Added expense with ID %s\n", expense.ID)
	return s.writeExpensesFile(s.filePath, data)
}

func (s *jsonStore) RemoveExpense(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	found := false
	newExpenses := make([]Expense, 0, len(data.Expenses)-1)
	for _, exp := range data.Expenses {
		if exp.ID != id {
			newExpenses = append(newExpenses, exp)
		} else {
			found = true
		}
	}
	if !found {
		log.Printf("Expense with ID %s not found\n", id)
		return fmt.Errorf("expense with ID %s not found", id)
	}
	log.Printf("Deleted expense with ID %s\n", id)
	data.Expenses = newExpenses
	return s.writeExpensesFile(s.filePath, data)
}

func (s *jsonStore) AddMultipleExpenses(expensesToAdd []Expense) error {
	if len(expensesToAdd) == 0 {
		return nil
	}
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	data.Expenses = append(data.Expenses, expensesToAdd...)
	log.Printf("Added %d new recurring expense instances\n", len(expensesToAdd))
	return s.writeExpensesFile(s.filePath, data)
}

func (s *jsonStore) RemoveMultipleExpenses(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	idsToRemove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idsToRemove[id] = struct{}{}
	}
	originalCount := len(data.Expenses)
	newExpenses := make([]Expense, 0, originalCount)
	for _, exp := range data.Expenses {
		if _, found := idsToRemove[exp.ID]; !found {
			newExpenses = append(newExpenses, exp)
		}
	}
	if len(newExpenses) == originalCount {
		log.Println("RemoveMultipleExpenses: no expenses found to remove")
		return nil
	}
	log.Printf("Removed %d expenses\n", originalCount-len(newExpenses))
	data.Expenses = newExpenses
	return s.writeExpensesFile(s.filePath, data)
}

func (s *jsonStore) UpdateExpense(id string, expense Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readExpensesFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %v", err)
	}
	found := false
	for i, exp := range data.Expenses {
		if exp.ID == id {
			data.Expenses[i] = expense
			data.Expenses[i].ID = id
			if data.Expenses[i].Currency == "" {
				data.Expenses[i].Currency = s.defaults["currency"]
			}
			found = true
			break
		}
	}
	if !found {
		log.Printf("expense with ID %s not found\n", id)
		return fmt.Errorf("expense with ID %s not found", id)
	}
	log.Printf("Edited expense with ID %s\n", id)
	return s.writeExpensesFile(s.filePath, data)
}
