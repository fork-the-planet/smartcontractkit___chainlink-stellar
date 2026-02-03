#[cfg(test)]
mod test {
    use crate::{ReentrancyGuard, GuardError};
    use soroban_sdk::{contract, contractimpl, Env};

    // A minimal test contract to provide contract context for testing
    #[contract]
    pub struct TestContract;

    #[contractimpl]
    impl TestContract {
        pub fn test_enter_exit(env: Env) -> bool {
            // Should be able to enter initially
            assert!(!ReentrancyGuard::is_entered(&env));
            assert!(ReentrancyGuard::enter(&env).is_ok());
            assert!(ReentrancyGuard::is_entered(&env));
            
            // Should fail to enter again
            assert_eq!(
                ReentrancyGuard::enter(&env),
                Err(GuardError::ReentrantCall)
            );
            
            // Should be able to exit
            ReentrancyGuard::exit(&env);
            assert!(!ReentrancyGuard::is_entered(&env));
            
            // Should be able to enter again after exit
            assert!(ReentrancyGuard::enter(&env).is_ok());
            ReentrancyGuard::exit(&env);
            
            true
        }

        pub fn test_with_guard_success(env: Env) -> i32 {
            let result: Result<i32, GuardError> = ReentrancyGuard::with_guard(&env, || {
                assert!(ReentrancyGuard::is_entered(&env));
                Ok(42)
            });
            
            assert_eq!(result, Ok(42));
            assert!(!ReentrancyGuard::is_entered(&env));
            
            result.unwrap()
        }

        pub fn test_with_guard_error(env: Env) -> bool {
            let result: Result<i32, GuardError> = ReentrancyGuard::with_guard(&env, || {
                Err(GuardError::ReentrantCall)
            });
            
            assert!(result.is_err());
            // Guard should still be released even on error
            assert!(!ReentrancyGuard::is_entered(&env));
            
            true
        }
    }

    #[test]
    fn test_guard_enter_exit() {
        let env = Env::default();
        let contract_id = env.register(TestContract, ());
        let client = TestContractClient::new(&env, &contract_id);
        
        assert!(client.test_enter_exit());
    }

    #[test]
    fn test_with_guard_success() {
        let env = Env::default();
        let contract_id = env.register(TestContract, ());
        let client = TestContractClient::new(&env, &contract_id);
        
        assert_eq!(client.test_with_guard_success(), 42);
    }

    #[test]
    fn test_with_guard_error() {
        let env = Env::default();
        let contract_id = env.register(TestContract, ());
        let client = TestContractClient::new(&env, &contract_id);
        
        assert!(client.test_with_guard_error());
    }
}
