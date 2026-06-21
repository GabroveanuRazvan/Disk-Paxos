{-# LANGUAGE DeriveGeneric #-}
import qualified Data.Map.Strict as M
import qualified Data.Set as S
import Data.List (intercalate, maximumBy, nub)
import Data.Ord (comparing)
import Data.Maybe (fromMaybe)
import System.Environment (getArgs)
import Control.Concurrent
import Control.Monad (forM)
import GHC.Generics (Generic)

type Proc = Int
type DiskId = Int
type Ballot = Int

data Val = NoInput | In Int
  deriving (Eq, Ord, Generic)

instance Show Val where
  show NoInput = "NotAnInput"
  show (In i)  = "in" ++ show i

data Block = Block
  { mbal :: Ballot
  , bal  :: Ballot
  , inp  :: Val
  } deriving (Eq, Ord, Generic)

instance Show Block where
  show (Block m b v) = "[mbal=" ++ show m ++ ", bal=" ++ show b ++ ", inp=" ++ show v ++ "]"

data ReadBlock = ReadBlock
  { rbBlock :: Block
  , rbProc  :: Proc
  } deriving (Eq, Ord, Show, Generic)

data Config = Config
  { cfgName                 :: String
  , nProc                   :: Int
  , inputsDomain            :: S.Set Val
  , diskDomain              :: S.Set DiskId
  , ballotCountPerProcess   :: Int
  } deriving Show

data State = State
  { input         :: M.Map Proc Val
  , output        :: M.Map Proc Val
  , disk          :: M.Map DiskId (M.Map Proc Block)
  , phase         :: M.Map Proc Int
  , dblock        :: M.Map Proc Block
  , disksWritten  :: M.Map Proc (S.Set DiskId)
  , blocksRead    :: M.Map Proc (M.Map DiskId (S.Set ReadBlock))
  , chosen        :: Val
  , allInput      :: S.Set Val
  } deriving (Eq, Ord, Generic)

instance Show State where
  show s = unlines
    [ "input        = " ++ showMap (input s)
    , "output       = " ++ showMap (output s)
    , "phase        = " ++ showMap (phase s)
    , "dblock       = " ++ showMap (dblock s)
    , "disksWritten = " ++ showMap (disksWritten s)
    , "chosen       = " ++ show (chosen s)
    , "allInput     = " ++ show (S.toList (allInput s))
    ]
    where
      showMap :: (Show k, Show v) => M.Map k v -> String
      showMap = show . M.toList

procs :: Config -> [Proc]
procs c = [1 .. nProc c]

initDB :: Block
initDB = Block 0 0 NoInput

ballot :: Config -> Proc -> S.Set Ballot
ballot c p =
  let start = p * ballotCountPerProcess c
  in S.fromList [start .. start + ballotCountPerProcess c - 1]

allBallots :: Config -> S.Set Ballot
allBallots c = S.unions [ballot c p | p <- procs c]

isMajority :: Config -> S.Set DiskId -> Bool
isMajority c s = S.size s * 2 > nProc c

getM :: (Ord k, Show k) => k -> M.Map k v -> v
getM k m = fromMaybe (error ("missing key: " ++ show k)) (M.lookup k m)

putM :: Ord k => k -> v -> M.Map k v -> M.Map k v
putM = M.insert

updM :: Ord k => k -> (v -> v) -> M.Map k v -> M.Map k v
updM k f m = M.adjust f k m

cartesianAssignments :: Ord k => [k] -> [v] -> [M.Map k v]
cartesianAssignments [] _ = [M.empty]
cartesianAssignments (k:ks) vs = [M.insert k v rest | v <- vs, rest <- cartesianAssignments ks vs]

subsets :: Ord a => S.Set a -> [S.Set a]
subsets s = map S.fromList (go (S.toList s))
  where
    go []     = [[]]
    go (x:xs) = let r = go xs in r ++ map (x:) r

hasRead :: State -> Proc -> DiskId -> Proc -> Bool
hasRead s p d q = any ((== q) . rbProc) $ S.toList $ getM d $ getM p (blocksRead s)

allBlocksRead :: Config -> State -> Proc -> S.Set Block
allBlocksRead c s p =
  S.fromList [ rbBlock br
             | d <- S.toList (diskDomain c)
             , br <- S.toList (getM d (getM p (blocksRead s))) ]

initializePhase :: Config -> Proc -> State -> State
initializePhase c p s =
  s { disksWritten = putM p S.empty (disksWritten s)
    , blocksRead   = putM p (M.fromList [(d, S.empty) | d <- S.toList (diskDomain c)]) (blocksRead s)
    }

setDblockFieldMbal :: Proc -> Ballot -> M.Map Proc Block -> M.Map Proc Block
setDblockFieldMbal p b = updM p (\blk -> blk {mbal = b})

setDblockPhase1Result :: Proc -> Ballot -> Val -> M.Map Proc Block -> M.Map Proc Block
setDblockPhase1Result p b v = updM p (\blk -> blk {bal = b, inp = v})


initialStates :: Config -> [State]
initialStates c =
  [ State
      { input = inpMap
      , output = M.fromList [(p, NoInput) | p <- ps]
      , disk = M.fromList [(d, M.fromList [(p, initDB) | p <- ps]) | d <- ds]
      , phase = M.fromList [(p, 0) | p <- ps]
      , dblock = M.fromList [(p, initDB) | p <- ps]
      , disksWritten = M.fromList [(p, S.empty) | p <- ps]
      , blocksRead = M.fromList [(p, M.fromList [(d, S.empty) | d <- ds]) | p <- ps]
      , chosen = NoInput
      , allInput = S.fromList (M.elems inpMap)
      }
  | inpMap <- cartesianAssignments ps (S.toList (inputsDomain c))
  ]
  where
    ps = procs c
    ds = S.toList (diskDomain c)


startBallot :: Config -> Proc -> State -> [State]
startBallot c p s
  | getM p (phase s) `notElem` [1,2] = []
  | otherwise =
      [ initializePhase c p $ s
          { phase = putM p 1 (phase s)
          , dblock = setDblockFieldMbal p b (dblock s)
          }
      | b <- S.toList (ballot c p)
      , b > mbal (getM p (dblock s))
      ]

phase1or2Write :: Config -> Proc -> DiskId -> State -> [State]
phase1or2Write _ p d s
  | getM p (phase s) `notElem` [1,2] = []
  | otherwise =
      [ s { disk = updM d (putM p (getM p (dblock s))) (disk s)
          , disksWritten = updM p (S.insert d) (disksWritten s)
          }
      ]

phase1or2Read :: Config -> Proc -> DiskId -> Proc -> State -> [State]
phase1or2Read c p d q s
  | q == p = []
  | S.notMember d (getM p (disksWritten s)) = []
  | mbal diskBlockQ < mbal dblockP =
      [ s { blocksRead = updM p (updM d (S.insert (ReadBlock diskBlockQ q))) (blocksRead s) } ]
  | otherwise = startBallot c p s
  where
    diskBlockQ = getM q (getM d (disk s))
    dblockP = getM p (dblock s)

endPhase1or2 :: Config -> Proc -> State -> [State]
endPhase1or2 c p s
  | not majorityReady = []
  | ph == 1 =
      [ initializePhase c p $ s
          { dblock = setDblockPhase1Result p (mbal (getM p (dblock s))) v (dblock s)
          , phase = putM p (ph + 1) (phase s)
          }
      | v <- phase1ChosenInputs
      ]
  | ph == 2 =
      [ initializePhase c p $ s
          { output = putM p (inp (getM p (dblock s))) (output s)
          , phase = putM p (ph + 1) (phase s)
          }
      ]
  | otherwise = []
  where
    ph = getM p (phase s)
    ps = procs c
    majorityReady = isMajority c $ S.fromList
      [ d | d <- S.toList (getM p (disksWritten s))
          , all (hasRead s p d) [q | q <- ps, q /= p]
      ]
    blocksSeen = S.insert (getM p (dblock s)) (allBlocksRead c s p)
    nonInit = S.filter ((/= NoInput) . inp) blocksSeen
    maxBal = if S.null nonInit then 0 else maximum (map bal (S.toList nonInit))
    phase1ChosenInputs
      | S.null nonInit = [getM p (input s)]
      | otherwise = nub [inp b | b <- S.toList nonInit, bal b == maxBal]

failProc :: Config -> Proc -> State -> [State]
failProc c p s =
  [ initializePhase c p $ s
      { input = putM p ip (input s)
      , phase = putM p 0 (phase s)
      , dblock = putM p initDB (dblock s)
      , output = putM p NoInput (output s)
      }
  | ip <- S.toList (inputsDomain c)
  ]

phase0Read :: Config -> Proc -> DiskId -> State -> [State]
phase0Read _ p d s
  | getM p (phase s) /= 0 = []
  | otherwise =
      let ownBlock = getM p (getM d (disk s))
      in [ s { blocksRead = updM p (updM d (S.insert (ReadBlock ownBlock p))) (blocksRead s) } ]

endPhase0 :: Config -> Proc -> State -> [State]
endPhase0 c p s
  | getM p (phase s) /= 0 = []
  | not majorityReady = []
  | S.null readBlocks = []
  | otherwise =
      [ initializePhase c p $ s
          { dblock = putM p (r {mbal = b}) (dblock s)
          , phase = putM p 1 (phase s)
          }
      | b <- S.toList (ballot c p)
      , all (\r -> b > mbal r) (S.toList readBlocks)
      , r <- maxBalBlocks
      ]
  where
    majorityReady = isMajority c $ S.fromList
      [ d | d <- S.toList (diskDomain c), hasRead s p d p ]
    readBlocks = allBlocksRead c s p
    maxBalVal = maximum (map bal (S.toList readBlocks))
    maxBalBlocks = [r | r <- S.toList readBlocks, bal r == maxBalVal]

nextRaw :: Config -> State -> [State]
nextRaw c s = nub $ concat
  [ startBallot c p s
  ++ concat [ phase0Read c p d s
            ++ phase1or2Write c p d s
            ++ concat [phase1or2Read c p d q s | q <- ps, q /= p]
             | d <- ds]
  ++ endPhase1or2 c p s
  ++ failProc c p s
  ++ endPhase0 c p s
  | p <- ps]
  where
    ps = procs c
    ds = S.toList (diskDomain c)

hNext :: Config -> State -> [State]
hNext c s = [ updateHistory s s' | s' <- nextRaw c s ]
  where
    updateHistory old new =
      let hasOutput p = getM p (output new) /= NoInput
          newChosen = if chosen old /= NoInput || all (not . hasOutput) (procs c)
                      then chosen old
                      else getM (head [p | p <- procs c, hasOutput p]) (output new)
          newAllInput = S.union (allInput old) (S.fromList (M.elems (input new)))
      in new { chosen = newChosen, allInput = newAllInput }

blocksOf :: Config -> State -> Proc -> S.Set Block
blocksOf c s p = S.unions
  [ S.singleton (getM p (dblock s))
  , S.fromList [getM p (getM d (disk s)) | d <- S.toList (diskDomain c)]
  , S.fromList [ rbBlock br
               | q <- procs c
               , d <- S.toList (diskDomain c)
               , br <- S.toList (getM d (getM q (blocksRead s)))
               , rbProc br == p ]
  ]

allBlocks :: Config -> State -> S.Set Block
allBlocks c s = S.unions [blocksOf c s p | p <- procs c]

majoritySets :: Config -> [S.Set DiskId]
majoritySets c = filter (isMajority c) (subsets (diskDomain c))

hInv1 :: Config -> State -> Bool
hInv1 c s = and
  [ all (`S.member` inputsDomain c) (M.elems (input s))
  , all (\v -> v == NoInput || S.member v (inputsDomain c)) (M.elems (output s))
  , all (`elem` [0..3]) (M.elems (phase s))
  , all (\v -> S.member v (inputsDomain c)) (S.toList (allInput s))
  , chosen s == NoInput || S.member (chosen s) (inputsDomain c)
  , M.keysSet (input s) == S.fromList (procs c)
  , M.keysSet (output s) == S.fromList (procs c)
  , M.keysSet (phase s) == S.fromList (procs c)
  , M.keysSet (dblock s) == S.fromList (procs c)
  , M.keysSet (disk s) == diskDomain c
  ]

validBlockFor :: Config -> State -> Proc -> Block -> Bool
validBlockFor c s p bk = and
  [ mbal bk `S.member` S.insert 0 (ballot c p)
  , bal bk  `S.member` S.insert 0 (ballot c p)
  , (bal bk == 0) == (inp bk == NoInput)
  , mbal bk >= bal bk
  , inp bk == NoInput || S.member (inp bk) (allInput s)
  ]

hInv2 :: Config -> State -> Bool
hInv2 c s = and
  [ all (\p -> all (validBlockFor c s p) (S.toList (blocksOf c s p))) ps
  , and [ if S.member d (getM p (disksWritten s))
          then getM p (phase s) `elem` [1,2] && getM p (getM d (disk s)) == getM p (dblock s)
          else True
        | p <- ps, d <- ds]
  , and [ if getM p (phase s) `elem` [1,2] && not (S.null (getM d (getM p (blocksRead s))))
          then S.member d (getM p (disksWritten s)) && not (hasRead s p d p)
          else True
        | p <- ps, d <- ds]
  , and [ if getM p (phase s) == 0
          then getM p (dblock s) == initDB
            && S.null (getM p (disksWritten s))
            && and [ rbProc br == p && rbBlock br == getM p (getM d (disk s))
                   | d <- ds, br <- S.toList (getM d (getM p (blocksRead s))) ]
          else True
        | p <- ps]
  , and [ if getM p (phase s) /= 0
          then mbal (getM p (dblock s)) `S.member` ballot c p
            && bal (getM p (dblock s)) `S.member` S.insert 0 (ballot c p)
            && and [ mbal (rbBlock br) < mbal (getM p (dblock s))
                   | d <- ds, br <- S.toList (getM d (getM p (blocksRead s))) ]
          else True
        | p <- ps]
  , and [ if getM p (phase s) `elem` [2,3]
          then bal (getM p (dblock s)) == mbal (getM p (dblock s))
          else True
        | p <- ps]
  , and [ getM p (output s) == if getM p (phase s) == 3 then inp (getM p (dblock s)) else NoInput
        | p <- ps]
  , chosen s == NoInput || S.member (chosen s) (allInput s)
  , and [ S.member (getM p (input s)) (allInput s) | p <- ps]
  , if chosen s == NoInput then all (\p -> getM p (output s) == NoInput) ps else True
  ]
  where
    ps = procs c
    ds = S.toList (diskDomain c)

hInv3 :: Config -> State -> Bool
hInv3 c s = and
  [ condition p q d | p <- ps, q <- ps, d <- ds]
  where
    ps = procs c
    ds = S.toList (diskDomain c)
    condition p q d =
      let prem = getM p (phase s) `elem` [1,2]
              && getM q (phase s) `elem` [1,2]
              && hasRead s p d q
              && hasRead s q d p
          left  = S.member (ReadBlock (getM q (dblock s)) q) (getM d (getM p (blocksRead s)))
          right = S.member (ReadBlock (getM p (dblock s)) p) (getM d (getM q (blocksRead s)))
      in not prem || left || right

hInv4 :: Config -> State -> Bool
hInv4 c s = and [condition p | p <- ps]
  where
    ps = procs c
    ds = S.toList (diskDomain c)
    majors = majoritySets c
    condition p =
      let dbp = getM p (dblock s)
      in and
      [ if getM p (phase s) /= 0
        then all (\bk -> mbal dbp >= bal bk) (S.toList (blocksOf c s p))
          && all (\dset -> any (\d -> let diskBlk = getM p (getM d (disk s))
                                      in mbal dbp >= mbal diskBlk && bal dbp >= bal diskBlk)
                                (S.toList dset)) majors
        else True
      , if getM p (phase s) == 1
        then all (\bk -> mbal (getM p (dblock s)) > bal bk) (S.toList (blocksOf c s p))
        else True
      , if getM p (phase s) `elem` [2,3]
        then any (\dset -> all (\d -> mbal (getM p (getM d (disk s))) == bal (getM p (dblock s))) (S.toList dset)) majors
        else True
      , all (\bk -> any (\dset -> all (\d -> mbal (getM p (getM d (disk s))) >= bal bk) (S.toList dset)) majors) (S.toList (blocksOf c s p))
      ]

maxBalInp :: Config -> State -> Ballot -> Val -> Bool
maxBalInp c s b v = all (\bk -> bal bk < b || inp bk == v) (S.toList (allBlocks c s))

valueChosen :: Config -> State -> Val -> Bool
valueChosen c s v = any okBallot (S.toList (allBallots c))
  where
    ps = procs c
    majors = majoritySets c
    okBallot b = maxBalInp c s b v && any (okPD b) [(p,dset) | p <- ps, dset <- majors]
    okPD b (p,dset) = all (\d -> bal (getM p (getM d (disk s))) >= b && readCondition b d p) (S.toList dset)
    readCondition b d p = all (\q ->
      let prem = getM q (phase s) == 1
              && mbal (getM q (dblock s)) >= b
              && hasRead s q d p
      in not prem || any (\br -> bal (rbBlock br) >= b) (S.toList (getM d (getM q (blocksRead s))))) ps

hInv6 :: Config -> State -> Bool
hInv6 c s = and
  [ chosen s == NoInput || valueChosen c s (chosen s)
  , all (\p -> getM p (output s) == chosen s || getM p (output s) == NoInput) (procs c)
  ]

allInvariants :: Config -> State -> [(String, Bool)]
allInvariants c s =
  [ ("HInv1", hInv1 c s)
  , ("HInv2", hInv2 c s)
  , ("HInv3", hInv3 c s)
  , ("HInv4", hInv4 c s)
  , ("HInv6", hInv6 c s)
  ]

checkState :: Config -> State -> Either String ()
checkState c s =
  case [name | (name, ok) <- allInvariants c s, not ok] of
    [] -> Right ()
    bad -> Left ("Invariant violation: " ++ intercalate ", " bad ++ "\n" ++ show s)

checkAndNext :: Config -> State -> Either String [State]
checkAndNext c s =
  case checkState c s of
    Left err -> Left err
    Right () -> Right (hNext c s)

chunksOf :: Int -> [a] -> [[a]]
chunksOf _ [] = []
chunksOf n xs =
  let (a, b) = splitAt (max 1 n) xs
  in a : chunksOf n b

parallelMapIO :: Int -> (a -> IO b) -> [a] -> IO [b]
parallelMapIO workers f xs
  | workers <= 1 = mapM f xs
  | null xs = pure []
  | otherwise = do
      let chunkSize = max 1 ((length xs + workers - 1) `div` workers)
          chunks = chunksOf chunkSize xs
      vars <- forM chunks $ \chunk -> do
        v <- newEmptyMVar
        _ <- forkIO $ do
          ys <- mapM f chunk
          putMVar v ys
        pure v
      fmap concat (mapM takeMVar vars)

mergeNext :: [Either String [State]] -> Either String [State]
mergeNext xs =
  case [err | Left err <- xs] of
    (err:_) -> Left err
    []      -> Right (concat [ns | Right ns <- xs])

exploreParallel :: Config -> Int -> Int -> IO ()
exploreParallel c maxStates workers = go S.empty (initialStates c) 0 0
  where
    go seen frontier count depth
      | null frontier = do
          putStrLn ("OK. Exhausted reachable state graph. States checked: " ++ show count)
          putStrLn ("Depth reached: " ++ show (max 0 (depth - 1)))
      | count >= maxStates = do
          putStrLn ("OK so far. Stopped at limit. States checked: " ++ show count)
          putStrLn ("Current frontier size: " ++ show (length frontier))
          putStrLn ("Depth reached: " ++ show (max 0 (depth - 1)))
      | otherwise = do
          let todo = filter (`S.notMember` seen) frontier
              seen' = foldr S.insert seen todo
              count' = count + length todo
          if null todo
            then go seen' [] count' depth
            else do
              checked <- parallelMapIO workers (pure . checkAndNext c) todo
              case mergeNext checked of
                Left err -> putStrLn err
                Right ns -> do
                  let nextFrontier = S.toList $ S.fromList $ filter (`S.notMember` seen') ns
                  go seen' nextFrontier count' (depth + 1)


configs :: M.Map String Config
configs = M.fromList
  [ ("trivial", Config "MC_HDiskSynod_trivial.cfg" 1 (S.fromList [In 1])     (S.fromList [1])   1)
  , ("small",   Config "MC_HDiskSynod_small.cfg"   2 (S.fromList [In 1,In 2]) (S.fromList [1,2]) 1)
  , ("medium",  Config "MC_HDiskSynod_medium.cfg"  2 (S.fromList [In 1,In 2]) (S.fromList [1,2]) 2)
  , ("3proc_min", Config "MC_HDiskSynod_3proc_min.cfg" 3 (S.fromList [In 1]) (S.fromList [1,2]) 1)
  , ("main",    Config "MC_HDiskSynod.cfg"         3 (S.fromList [In 1,In 2]) (S.fromList [1,2]) 2)
  ]

main :: IO ()
main = do
  args <- getArgs
  let key = case args of
              []    -> "small"
              (x:_) -> x
      limit = case args of
                (_:n:_) -> read n
                _       -> 50000
      workers = case args of
                  (_:_:w:_) -> read w
                  _         -> 4
  case M.lookup key configs of
    Nothing -> do
      putStrLn "Unknown config. Use one of: trivial, small, medium, 3proc_min, main"
    Just c -> do
      putStrLn ("Running " ++ cfgName c ++ " with state limit " ++ show limit)
      putStrLn ("Worker threads: " ++ show workers)
      putStrLn ("Initial states: " ++ show (length (initialStates c)))
      exploreParallel c limit workers
