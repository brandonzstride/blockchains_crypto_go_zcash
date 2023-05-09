# Zcash Shielded Transactions Benchmarking

For now, we will focus on Zcash's shielded transactions. If time permits, we may extend our scope to include both shielded and unshielded transactions.

## Steps For Now

1. **Understand Zcash and Diablo**: Study up shielded transactions  in Zcash. Also, familiarize yourself with the Diablo benchmarking tool.

To interact with Zcash and create shielded transactions, you would typically use a Zcash full node, such as zcashd. Here's an example of how you might create a shielded transaction using the zcash-cli command-line tool:

```
zcash-cli z_sendmany "from_address" "[{\"amount\": 1.23, \"address\": \"to_address\"}]"

```

2. **transaction scenarios**: Our focus will be on scenarios involving shielded transactions. This could include the creation and transmission of shielded transactions, or the conversion from transparent to shielded transactions, etc.

3. **Find relevant Zcash files**: The relevant C++ files for shielded transactions in Zcash are in the src directory of the Zcash GitHub repository.
    * `src/zcash`: This directory contains most of the logic for Zcash's privacy features. For example, the `src/zcash/JoinSplit.cpp` file contains the implementation of the JoinSplit operation, which is crucial for shielded transactions.
    * `src/wallet/asyncrpcoperation_sendmany.cpp`: This file contains the logic for creating and sending shielded transactions from a wallet.

4. **Designing the workload**: Our benchmarking scenario with will focus on shielded transactions. This might involve creating and sending a series of shielded transactions and measuring the time or resources consumed.
In this example, workload.json is a configuration file that specifies the workload to be run.

```
diablo --benchmark --workload workload.json
```

5. **Implement the workload in C++**: Utilize the Zcash C++ APIs to implement the benchmark. This will likely involve calling Zcash functions to create and send shielded transactions, and time taken. Check Later
 (<https://github.com/zcash/zcash/pull/4986>)
 (<https://github.com/zcash/zcash/blob/master/doc/zmq.md>)

6. **Analyze the  results**: Running the benchmark and collect the results. Look for any patterns or anomalies that might provide insights into the performance of shielded transactions in Zcash.

## Note

This benchmarking project is currently focusing on shielded transactions in Zcash. However, the scope could be extended based on future needs or if time permits.
